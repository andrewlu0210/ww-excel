package xlsxfile

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/parser"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// Columns scanned for custom widths and styles beyond the used range.
const scanCols = 1024

// Read loads an xlsx file. The returned excelize file is kept as the
// base for in-place saves; formulas are loaded as text (with cached
// values) and compiled by the caller.
func Read(path string) (*model.Workbook, *excelize.File, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, nil, err
	}
	raw, err := rawCells(path)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	wb, err := load(f, raw)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return wb, f, nil
}

type reader struct {
	f      *excelize.File
	wb     *model.Workbook
	styles map[int]uint32
	// Every cell element per sheet (see rawCells).
	raw map[string]map[[2]int]rawCell
}

func load(f *excelize.File, raw map[string]map[[2]int]rawCell) (*model.Workbook, error) {
	def, _ := f.GetStyle(0)
	defStyle := fromExcelize(f, def)
	if defStyle.Font.Name == "" {
		defStyle.Font = model.DefaultFont
	}
	r := &reader{f: f, wb: &model.Workbook{Styles: model.NewStyleTable(defStyle)}, styles: map[int]uint32{0: 0}, raw: raw}
	for _, name := range f.GetSheetList() {
		sh, err := r.sheet(name)
		if err != nil {
			return nil, fmt.Errorf("工作表「%s」：%w", name, err)
		}
		r.wb.Sheets = append(r.wb.Sheets, sh)
	}
	if len(r.wb.Sheets) == 0 {
		return nil, fmt.Errorf("活頁簿沒有工作表")
	}
	return r.wb, nil
}

func (r *reader) style(id int) uint32 {
	if s, ok := r.styles[id]; ok {
		return s
	}
	xs, err := r.f.GetStyle(id)
	var st model.Style
	if err == nil {
		st = fromExcelize(r.f, xs)
	}
	def := r.wb.Styles.Default()
	if st.Font.Name == "" {
		st.Font.Name = def.Font.Name
	}
	if st.Font.Size == 0 {
		st.Font.Size = def.Font.Size
	}
	s := r.wb.Styles.Intern(st)
	r.styles[id] = s
	return s
}

func (r *reader) sheet(name string) (*model.Sheet, error) {
	sh := model.NewSheet(name)
	sh.SourceName = name
	l := &sh.Layout
	if props, err := r.f.GetSheetProps(name); err == nil {
		if props.DefaultRowHeight != nil && *props.DefaultRowHeight > 0 {
			l.DefaultRow = ptToPx(*props.DefaultRowHeight)
		}
		if props.DefaultColWidth != nil && *props.DefaultColWidth > 0 {
			l.DefaultCol = widthToPx(*props.DefaultColWidth)
		}
	}

	rows, err := r.f.Rows(name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	maxCol := 0
	for row := 0; rows.Next(); row++ {
		if row >= model.MaxRows {
			break
		}
		opts := rows.GetRowOpts()
		if opts.Height > 0 && ptToPx(opts.Height) != l.DefaultRow || opts.Hidden {
			px := l.DefaultRow
			if opts.Height > 0 {
				px = ptToPx(opts.Height)
			}
			l.Rows[row] = model.Size{Px: px, Hidden: opts.Hidden}
		}
		if opts.StyleID > 0 {
			l.RowStyles[row] = r.style(opts.StyleID)
		}
		cols, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, err
		}
		maxCol = max(maxCol, len(cols))
		for col, raw := range cols {
			if raw == "" {
				continue // formulas with empty results are found below
			}
			if err := r.cell(sh, row, col, raw); err != nil {
				return nil, err
			}
		}
	}
	if err := rows.Error(); err != nil {
		return nil, err
	}
	// Styled cells without a value, and formulas whose cached value is
	// empty, come from the raw cell list.
	for pos := range r.raw[name] {
		if pos[0] >= model.MaxRows || pos[1] >= model.MaxCols || sh.Get(pos[0], pos[1]).HasContent() {
			continue
		}
		if err := r.cell(sh, pos[0], pos[1], ""); err != nil {
			return nil, err
		}
	}

	for col := 0; col < max(maxCol, scanCols) && col < model.MaxCols; col++ {
		colName, _ := excelize.ColumnNumberToName(col + 1)
		w, err := r.f.GetColWidth(name, colName)
		if err != nil {
			return nil, err
		}
		visible, _ := r.f.GetColVisible(name, colName)
		if px := widthToPx(w); px != l.DefaultCol || !visible {
			l.Cols[col] = model.Size{Px: px, Hidden: !visible}
		}
		if id, err := r.f.GetColStyle(name, colName); err == nil && id > 0 {
			l.ColStyles[col] = r.style(id)
		}
	}

	merges, err := r.f.GetMergeCells(name)
	if err != nil {
		return nil, err
	}
	for _, m := range merges {
		c0, r0, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
		c1, r1, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
		if err1 == nil && err2 == nil {
			l.Merges = append(l.Merges, model.Range{R0: r0 - 1, C0: c0 - 1, R1: r1 - 1, C1: c1 - 1}.Normalize())
		}
	}
	l.SortMerges()

	if panes, err := r.f.GetPanes(name); err == nil && panes.Freeze {
		l.FreezeRows, l.FreezeCols = max(panes.YSplit, 0), max(panes.XSplit, 0)
	}
	if names := r.f.GetDefinedName(); len(names) > 0 {
		for _, dn := range names {
			if dn.Name != "_xlnm._FilterDatabase" || dn.Scope != name {
				continue
			}
			ref := dn.RefersTo
			if i := strings.LastIndex(ref, "!"); i >= 0 {
				ref = ref[i+1:]
			}
			ref = strings.ReplaceAll(ref, "$", "")
			parts := strings.Split(ref, ":")
			if len(parts) != 2 {
				continue
			}
			c0, r0, err1 := excelize.CellNameToCoordinates(parts[0])
			c1, r1, err2 := excelize.CellNameToCoordinates(parts[1])
			if err1 == nil && err2 == nil {
				l.Filter = &model.AutoFilter{Range: model.Range{R0: r0 - 1, C0: c0 - 1, R1: r1 - 1, C1: c1 - 1}.Normalize(), Cols: map[int][]string{}}
			}
		}
	}
	return sh, nil
}

func (r *reader) cell(sh *model.Sheet, row, col int, raw string) error {
	name, _ := excelize.CoordinatesToCellName(col+1, row+1)
	sheet := sh.Name
	info := r.raw[sheet][[2]int{row, col}]
	styleID := info.style
	formula := ""
	if info.formula {
		var err error
		if formula, err = r.f.GetCellFormula(sheet, name); err != nil {
			return err
		}
	}
	if raw == "" && formula == "" {
		if styleID > 0 {
			sh.Set(row, col, model.Cell{S: r.style(styleID)})
		}
		return nil
	}
	typ, err := r.f.GetCellType(sheet, name)
	if err != nil {
		return err
	}
	cell := value(typ, raw)
	if formula != "" {
		// Canonical text, without the file's _xlfn. prefixes.
		if node, err := parser.Parse(formula); err == nil {
			formula = ast.Format(node)
		}
		cell.F = &model.Formula{Text: formula}
	}
	if styleID > 0 {
		cell.S = r.style(styleID)
	}
	sh.Set(row, col, cell)
	return nil
}

func value(typ excelize.CellType, raw string) model.Cell {
	switch typ {
	case excelize.CellTypeBool:
		return model.BoolCell(raw == "1" || strings.EqualFold(raw, "TRUE"))
	case excelize.CellTypeError:
		if code := model.ErrorCode(raw); code != "" {
			return model.ErrorCell(code)
		}
		return model.StringCell(raw)
	case excelize.CellTypeSharedString, excelize.CellTypeInlineString, excelize.CellTypeFormula:
		return model.StringCell(raw)
	case excelize.CellTypeDate:
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			return model.NumberCell(numfmt.TimeSerial(t))
		}
		return model.StringCell(raw)
	}
	// Untyped cells are numbers.
	if n, err := strconv.ParseFloat(raw, 64); err == nil {
		return model.NumberCell(n)
	}
	if raw == "" {
		return model.Cell{}
	}
	return model.StringCell(raw)
}
