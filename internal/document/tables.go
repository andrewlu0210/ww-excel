package document

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
)

// DefaultTableStyle is Excel's default table style (blue, medium).
const DefaultTableStyle = "TableStyleMedium2"

// rebuild is a step that only asks for formulas to be re-resolved
// (tables changed, so structured references point elsewhere).
func rebuild(sheet int) command.Command {
	eff := func(*model.Workbook) command.Effect { return command.Effect{Sheet: sheet, Names: true} }
	return &command.Func{Name: "", DoFn: eff, UndoFn: eff}
}

// NextTableName returns the first unused 表格N.
func (d *Document) NextTableName() string {
	for n := 1; ; n++ {
		name := fmt.Sprintf("表格%d", n)
		if t, _ := d.wb.FindTable(name); t == nil && d.wb.LookupName(name, nil) == nil {
			return name
		}
	}
}

// CreateTableCommand turns rg into a table. Without headers, a header
// row (欄1, 欄2, ...) is inserted above the data within the range's
// columns.
func (d *Document) CreateTableCommand(sheet int, rg model.Range, hasHeaders bool, style string) (command.Command, string, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, "", errors.New("無效的工作表")
	}
	sh := d.wb.Sheets[sheet]
	rg = rg.Clamp()
	if rows, _ := sh.UsedRange(); rg.R1 >= rows && rg.R1-rg.R0 > 1000 {
		rg.R1 = max(rg.R0, rows-1)
	}
	if rg.C1-rg.C0 > 500 {
		return nil, "", errors.New("表格的欄太多")
	}
	for _, t := range sh.Layout.Tables {
		if t.Range.Overlaps(rg) {
			return nil, "", errors.New("表格不能與其他表格重疊")
		}
	}
	for _, m := range sh.Layout.Merges {
		if m.Overlaps(rg) {
			return nil, "", errors.New("表格中不能有合併儲存格")
		}
	}
	if style == "" {
		style = DefaultTableStyle
	}
	var changes []command.CellChange
	if !hasHeaders {
		// Move the data down one row to make room for the header.
		if rg.R1+1 >= model.MaxRows {
			return nil, "", errors.New("工作表沒有空間插入標題列")
		}
		for c := rg.C0; c <= rg.C1; c++ {
			if sh.Get(rg.R1+1, c).HasContent() {
				return nil, "", errors.New("無法插入標題列：資料下方的儲存格不是空白")
			}
		}
		for r := rg.R1; r >= rg.R0; r-- {
			for c := rg.C0; c <= rg.C1; c++ {
				cell := sh.Get(r, c)
				if cell.F != nil {
					node := ast.Translate(d.calc.Parsed(cell.F, sheet), 1, 0)
					cell.F = d.calc.CompileNode(node, sheet)
				}
				changes = append(changes, command.CellChange{Row: r + 1, Col: c, After: cell})
			}
		}
		for c := rg.C0; c <= rg.C1; c++ {
			changes = append(changes, command.CellChange{Row: rg.R0, Col: c, After: model.StringCell(fmt.Sprintf("欄%d", c-rg.C0+1))})
		}
		rg.R1++
	} else {
		// Headers must be unique, non-empty text.
		seen := map[string]bool{}
		for c := rg.C0; c <= rg.C1; c++ {
			cell := sh.Get(rg.R0, c)
			name := cell.Display()
			base := name
			if base == "" {
				base = fmt.Sprintf("欄%d", c-rg.C0+1)
			}
			name = base
			for i := 2; seen[strings.ToLower(name)]; i++ {
				name = base + strconv.Itoa(i)
			}
			seen[strings.ToLower(name)] = true
			if name != cell.Display() || cell.Kind != model.KindString || cell.F != nil {
				changes = append(changes, command.CellChange{Row: rg.R0, Col: c, After: model.Cell{Kind: model.KindString, Str: name, S: cell.S}})
			}
		}
	}
	if rg.R1 == rg.R0 {
		rg.R1++ // a table has at least one data row
	}
	name := d.NextTableName()
	t := model.Table{Name: name, Range: rg, Style: style}
	after := sh.Layout.Clone()
	after.Tables = append(after.Tables, t)
	if after.Filter == nil {
		after.Filter = &model.AutoFilter{Range: rg, Cols: map[int][]string{}}
	}
	area := rg
	cmds := []command.Command{}
	if len(changes) > 0 {
		cmds = append(cmds, &command.SetCells{Sheet: sheet, Changes: changes})
	}
	cmds = append(cmds, &command.Layout{Sheet: sheet, After: after, Area: &area}, rebuild(sheet))
	return &command.Batch{Name: "建立表格", Cmds: cmds}, name, nil
}

// tableCommand edits one table's settings by name.
func (d *Document) tableCommand(sheet int, name, label string, edit func(t *model.Table) error) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	after := d.wb.Sheets[sheet].Layout.Clone()
	for i := range after.Tables {
		if strings.EqualFold(after.Tables[i].Name, name) {
			if err := edit(&after.Tables[i]); err != nil {
				return nil, err
			}
			return &command.Batch{Name: label, Cmds: []command.Command{
				&command.Layout{Sheet: sheet, After: after}, rebuild(sheet),
			}}, nil
		}
	}
	return nil, fmt.Errorf("表格「%s」不存在", name)
}

// SetTableStyleCommand changes a table's style and banding.
func (d *Document) SetTableStyleCommand(sheet int, name, style string, bands bool) (command.Command, error) {
	return d.tableCommand(sheet, name, "表格樣式", func(t *model.Table) error {
		t.Style, t.NoBands = style, !bands
		return nil
	})
}

// RenameTableCommand renames a table and the structured references to
// it.
func (d *Document) RenameTableCommand(sheet int, old, name string) (command.Command, error) {
	name = strings.TrimSpace(name)
	if !model.ValidName(name) {
		return nil, fmt.Errorf("「%s」不是有效的表格名稱", name)
	}
	if t, _ := d.wb.FindTable(name); t != nil && !strings.EqualFold(old, name) {
		return nil, fmt.Errorf("已經有名為「%s」的表格", name)
	}
	if d.wb.LookupName(name, nil) != nil {
		return nil, fmt.Errorf("已經有名為「%s」的名稱", name)
	}
	layout, err := d.tableCommand(sheet, old, "重新命名表格", func(t *model.Table) error {
		t.Name = name
		return nil
	})
	if err != nil {
		return nil, err
	}
	edits := d.rewriteFormulas(nil, func(n ast.Node) (ast.Node, bool) {
		return ast.MapTableRefs(n, func(r ast.TableRef) ast.Node {
			if strings.EqualFold(r.Table, old) {
				r.Table = name
			}
			return r
		})
	})
	return &command.Batch{Name: "重新命名表格", Cmds: []command.Command{layout, editsCommand(sheet, edits)}}, nil
}

// editsCommand applies formula rewrites as an undoable step.
func editsCommand(sheet int, edits []formulaEdit) command.Command {
	return &command.Func{
		DoFn: func(wb *model.Workbook) command.Effect {
			return command.Effect{Sheet: sheet, Others: applyEdits(wb, edits, false), Names: true}
		},
		UndoFn: func(wb *model.Workbook) command.Effect {
			return command.Effect{Sheet: sheet, Others: applyEdits(wb, edits, true), Names: true}
		},
	}
}

// ConvertTableCommand turns a table back into a normal range; formulas
// with structured references to it get normal references.
func (d *Document) ConvertTableCommand(sheet int, name string) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	sh := d.wb.Sheets[sheet]
	after := sh.Layout.Clone()
	idx := -1
	for i, t := range after.Tables {
		if strings.EqualFold(t.Name, name) {
			idx = i
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("表格「%s」不存在", name)
	}
	table := after.Tables[idx]
	after.Tables = append(after.Tables[:idx:idx], after.Tables[idx+1:]...)
	if after.Filter != nil && after.Filter.Range == table.Range {
		after.Filter = nil
	}
	// Rewrite references while the table still exists.
	var edits []formulaEdit
	for si, s := range d.wb.Sheets {
		s.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			if c.F == nil {
				return
			}
			node, changed := ast.MapTableRefs(d.calc.Parsed(c.F, si), func(r ast.TableRef) ast.Node {
				inTable := r.Table == "" && s == sh && table.Range.Contains(row, col)
				if !strings.EqualFold(r.Table, name) && !inTable {
					return r
				}
				ref, code := d.calc.Table(r, si, row, col)
				if code != "" {
					return ast.Error{Code: code}
				}
				a := ast.Area{Kind: ast.CellArea, R0: ref.R0, C0: ref.C0, R1: ref.R1, C1: ref.C1, AbsR0: true, AbsC0: true, AbsR1: true, AbsC1: true}
				if r.Table == "" || strings.HasPrefix(strings.TrimSpace(r.Spec), "@") {
					a.AbsR0, a.AbsR1 = false, false // this-row references stay relative
				}
				out := ast.Ref{Area: a}
				if ref.Sheet != si {
					out.Sheet = d.wb.Sheets[ref.Sheet].Name
				}
				return out
			})
			if changed {
				nc := c
				nc.F = d.calc.CompileNode(node, si)
				edits = append(edits, formulaEdit{sheet: s, row: row, col: col, before: c, after: nc})
			}
		})
	}
	return &command.Batch{Name: "轉換為範圍", Cmds: []command.Command{
		editsCommand(sheet, edits), &command.Layout{Sheet: sheet, After: after},
	}}, nil
}

// ToggleTotalsCommand adds or removes a table's totals row: 合計 in the
// first column and a SUBTOTAL of the last column.
func (d *Document) ToggleTotalsCommand(sheet int, name string) (command.Command, error) {
	t, sh := d.wb.FindTable(name)
	if t == nil || sh != d.wb.Sheets[sheet] {
		return nil, fmt.Errorf("表格「%s」不存在", name)
	}
	rg := t.Range
	var changes []command.CellChange
	if t.TotalsRow {
		sh.EachInRange(model.Range{R0: rg.R1, C0: rg.C0, R1: rg.R1, C1: rg.C1}, func(row, col int, c model.Cell) {
			changes = append(changes, command.CellChange{Row: row, Col: col, After: model.Cell{S: c.S}})
		})
	} else {
		row := rg.R1 + 1
		if row >= model.MaxRows {
			return nil, errors.New("工作表沒有空間加入合計列")
		}
		for c := rg.C0; c <= rg.C1; c++ {
			if sh.Get(row, c).HasContent() {
				return nil, errors.New("無法加入合計列：表格下方的儲存格不是空白")
			}
		}
		cols := t.Columns(sh)
		last := cols[len(cols)-1]
		f, err := d.calc.Compile(fmt.Sprintf("SUBTOTAL(109,%s)", ast.Format(ast.TableColumnRef(t.Name, last))), sheet)
		if err != nil {
			return nil, err
		}
		if rg.C1 > rg.C0 {
			changes = append(changes, command.CellChange{Row: row, Col: rg.C0, After: model.StringCell("合計")})
		}
		changes = append(changes, command.CellChange{Row: row, Col: rg.C1, After: model.Cell{F: f}})
	}
	layout, err := d.tableCommand(sheet, name, "合計列", func(tt *model.Table) error {
		if tt.TotalsRow {
			tt.Range.R1--
		} else {
			tt.Range.R1++
		}
		tt.TotalsRow = !tt.TotalsRow
		return nil
	})
	if err != nil {
		return nil, err
	}
	// The layout (and the table) changes first so new formulas resolve.
	cmds := []command.Command{layout}
	if len(changes) > 0 {
		cmds = append(cmds, &command.SetCells{Sheet: sheet, Changes: changes})
	}
	cmds = append(cmds, rebuild(sheet))
	return &command.Batch{Name: "合計列", Cmds: cmds}, nil
}

// ExpandTableCommand grows a table when a cell right below it or right
// next to it is filled in (Excel's auto-expansion). Returns nil if no
// table is adjacent to (row, col).
func (d *Document) ExpandTableCommand(sheet, row, col int) command.Command {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil
	}
	sh := d.wb.Sheets[sheet]
	for _, t := range sh.Layout.Tables {
		data := t.DataRows()
		below := !t.TotalsRow && row == t.Range.R1+1 && col >= t.Range.C0 && col <= t.Range.C1
		right := col == t.Range.C1+1 && row >= t.Range.R0 && row <= data.R1 && col < model.MaxCols
		if !below && !right {
			continue
		}
		var header *command.SetCells
		if right {
			for _, o := range sh.Layout.Tables {
				if o.Range.Contains(t.Range.R0, col) {
					return nil
				}
			}
			if h := sh.Get(t.Range.R0, col); !h.HasContent() || row == t.Range.R0 {
				if row != t.Range.R0 {
					name := fmt.Sprintf("欄%d", col-t.Range.C0+1)
					header = &command.SetCells{Sheet: sheet, Changes: []command.CellChange{{Row: t.Range.R0, Col: col, After: model.Cell{Kind: model.KindString, Str: name, S: h.S}}}}
				}
			}
		}
		cmd, err := d.tableCommand(sheet, t.Name, "擴充表格", func(tt *model.Table) error {
			if below {
				tt.Range.R1++
			} else {
				tt.Range.C1++
			}
			return nil
		})
		if err != nil {
			return nil
		}
		b := cmd.(*command.Batch)
		if f := sh.Layout.Filter; f != nil && f.Range == t.Range {
			// Keep the table's filter range in step.
			l := b.Cmds[0].(*command.Layout)
			nf := f.Clone()
			if below {
				nf.Range.R1++
			} else {
				nf.Range.C1++
			}
			l.After.Filter = nf
		}
		if header != nil {
			b.Cmds = append([]command.Command{header}, b.Cmds...)
		}
		return cmd
	}
	return nil
}
