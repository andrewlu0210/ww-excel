package xlsxfile

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/xuri/excelize/v2"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/parser"
	"ww-excel/internal/model"
)

// Changes tells Save what differs from the source file. Sheets without
// a source (new, or the whole workbook when there is no source file)
// are written completely.
type Changes struct {
	// Cells edited since load, per sheet.
	Cells map[*model.Sheet]map[[2]int]bool
	// Layout of each sheet as it was in the source file, for sheets whose
	// layout changed (already shifted by Shifts).
	OldLayouts map[*model.Sheet]model.Layout
	// Row/column insertions and deletions to replay on the source first,
	// in order, so untouched content (charts, comments) moves along.
	Shifts map[*model.Sheet][]Shift
	// Names: the defined names changed; rewrite them.
	Names bool
}

// Shift is an insertion (Count > 0) or deletion (Count < 0) of rows or
// columns at index At.
type Shift struct {
	Rows      bool
	At, Count int
}

// Save writes wb to path. src is the file the workbook was loaded from
// (nil for a new file); it is updated in place and returned as the new
// source. On error the file at path is left untouched.
func Save(path string, wb *model.Workbook, src *excelize.File, ch Changes) (*excelize.File, error) {
	f := src
	full := map[*model.Sheet]bool{}
	if f == nil {
		f = excelize.NewFile()
	}
	w := &writer{f: f, wb: wb, styles: map[uint32]int{}}
	if err := w.sheets(full); err != nil {
		return nil, err
	}
	for _, sh := range wb.Sheets {
		var err error
		if full[sh] || src == nil {
			err = w.fullSheet(sh)
		} else {
			err = w.changedSheet(sh, ch)
		}
		if err != nil {
			return nil, fmt.Errorf("工作表「%s」：%w", sh.Name, err)
		}
	}
	if ch.Names || src == nil {
		if err := w.names(); err != nil {
			return nil, err
		}
	}
	// No cached values are written for text results, so let Excel
	// recalculate on open.
	fullCalc := true
	if err := f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: &fullCalc}); err != nil {
		return nil, err
	}

	// excelize picks the format from the extension, so keep it.
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*"+filepath.Ext(path))
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)
	if err := f.SaveAs(tmpName); err != nil {
		return nil, err
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return nil, err
	}
	for _, sh := range wb.Sheets {
		sh.SourceName = sh.Name
	}
	return f, nil
}

type writer struct {
	f      *excelize.File
	wb     *model.Workbook
	styles map[uint32]int
}

// sheets makes the file's sheet list match the workbook: renames,
// additions, deletions and order. Sheets created here are marked full.
func (w *writer) sheets(full map[*model.Sheet]bool) error {
	f := w.f
	existing := map[string]bool{}
	for _, name := range f.GetSheetList() {
		existing[name] = true
	}
	// Rename kept sheets to temporary names first so swaps work.
	keep := map[string]bool{}
	tmp := func(i int) string { return fmt.Sprintf("__ww_%d", i) }
	for i, sh := range w.wb.Sheets {
		if sh.SourceName != "" && existing[sh.SourceName] && !keep[sh.SourceName] {
			keep[sh.SourceName] = true
			if err := f.SetSheetName(sh.SourceName, tmp(i)); err != nil {
				return err
			}
			continue
		}
		if _, err := f.NewSheet(tmp(i)); err != nil {
			return err
		}
		full[sh] = true
	}
	for name := range existing {
		if !keep[name] {
			if err := f.DeleteSheet(name); err != nil {
				return err
			}
		}
	}
	for i, sh := range w.wb.Sheets {
		if err := f.SetSheetName(tmp(i), sh.Name); err != nil {
			return err
		}
	}
	// Order: move each sheet before the one that should follow it.
	for i := len(w.wb.Sheets) - 2; i >= 0; i-- {
		if err := f.MoveSheet(w.wb.Sheets[i].Name, w.wb.Sheets[i+1].Name); err != nil {
			return err
		}
	}
	if idx, err := f.GetSheetIndex(w.wb.Sheets[0].Name); err == nil {
		f.SetActiveSheet(idx)
	}
	return nil
}

// tables replaces tables that changed. A totals row is written as a
// normal row below the table (excelize cannot write one).
func (w *writer) tables(sh *model.Sheet, old, cur []model.Table) error {
	same := func(a, b model.Table) bool { return a == b }
	find := func(list []model.Table, name string) (model.Table, bool) {
		for _, t := range list {
			if strings.EqualFold(t.Name, name) {
				return t, true
			}
		}
		return model.Table{}, false
	}
	for _, t := range old {
		if n, ok := find(cur, t.Name); !ok || !same(n, t) {
			if err := w.f.DeleteTable(t.Name); err != nil {
				return err
			}
		}
	}
	for _, t := range cur {
		if o, ok := find(old, t.Name); ok && same(o, t) {
			continue
		}
		rg := t.Range
		if t.TotalsRow {
			rg.R1--
		}
		stripes := !t.NoBands
		if err := w.f.AddTable(sh.Name, &excelize.Table{
			Range: formatSqref([]model.Range{rg}), Name: t.Name, StyleName: t.Style, ShowRowStripes: &stripes,
		}); err != nil {
			return fmt.Errorf("表格「%s」：%w", t.Name, err)
		}
	}
	return nil
}

// names replaces the file's defined names with the workbook's, keeping
// built-in ones (_xlnm.*: print areas, filter ranges).
func (w *writer) names() error {
	for _, dn := range w.f.GetDefinedName() {
		if strings.HasPrefix(dn.Name, "_xlnm.") {
			continue
		}
		scope := dn.Scope
		if scope == "Workbook" {
			scope = ""
		}
		if err := w.f.DeleteDefinedName(&excelize.DefinedName{Name: dn.Name, Scope: scope}); err != nil {
			return err
		}
	}
	for _, n := range w.wb.Names {
		dn := &excelize.DefinedName{Name: n.Name, RefersTo: n.Formula, Comment: n.Comment}
		if n.Scope != nil {
			dn.Scope = n.Scope.Name
		}
		if err := w.f.SetDefinedName(dn); err != nil {
			return fmt.Errorf("名稱「%s」：%w", n.Name, err)
		}
	}
	return nil
}

func (w *writer) style(id uint32) (int, error) {
	if id == 0 {
		return 0, nil
	}
	if x, ok := w.styles[id]; ok {
		return x, nil
	}
	x, err := w.f.NewStyle(toExcelize(w.wb.StyleTable().Get(id)))
	if err != nil {
		return 0, err
	}
	w.styles[id] = x
	return x, nil
}

func (w *writer) fullSheet(sh *model.Sheet) error {
	var err error
	sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
		if err == nil {
			err = w.cell(sh, row, col, c)
		}
	})
	if err != nil {
		return err
	}
	return w.layout(sh, model.NewLayout())
}

func (w *writer) changedSheet(sh *model.Sheet, ch Changes) error {
	for _, s := range ch.Shifts[sh] {
		if err := w.replayShift(sh.Name, s); err != nil {
			return err
		}
	}
	for pos := range ch.Cells[sh] {
		if err := w.cell(sh, pos[0], pos[1], sh.Get(pos[0], pos[1])); err != nil {
			return err
		}
	}
	if old, ok := ch.OldLayouts[sh]; ok {
		return w.layout(sh, old)
	}
	return nil
}

func (w *writer) replayShift(sheet string, s Shift) error {
	// excelize does not move comments; capture them first.
	comments, err := w.f.GetComments(sheet)
	if err != nil {
		return err
	}
	if err := w.shiftCells(sheet, s); err != nil {
		return err
	}
	for _, c := range comments {
		col, row, err := excelize.CellNameToCoordinates(c.Cell)
		if err != nil {
			continue
		}
		i := col - 1
		if s.Rows {
			i = row - 1
		}
		ni, keep := model.ShiftIndex(i, s.At, s.Count)
		if keep && ni == i {
			continue
		}
		if err := w.f.DeleteComment(sheet, c.Cell); err != nil {
			return err
		}
		if !keep {
			continue
		}
		if s.Rows {
			row = ni + 1
		} else {
			col = ni + 1
		}
		c.Cell, _ = excelize.CoordinatesToCellName(col, row)
		if err := w.f.AddComment(sheet, c); err != nil {
			return err
		}
	}
	return nil
}

func (w *writer) shiftCells(sheet string, s Shift) error {
	f := w.f
	if s.Rows {
		if s.Count > 0 {
			return f.InsertRows(sheet, s.At+1, s.Count)
		}
		for i := 0; i < -s.Count; i++ {
			if err := f.RemoveRow(sheet, s.At+1); err != nil {
				return err
			}
		}
		return nil
	}
	col, err := excelize.ColumnNumberToName(s.At + 1)
	if err != nil {
		return err
	}
	if s.Count > 0 {
		return f.InsertCols(sheet, col, s.Count)
	}
	for i := 0; i < -s.Count; i++ {
		if err := f.RemoveCol(sheet, col); err != nil {
			return err
		}
	}
	return nil
}

func (w *writer) cell(sh *model.Sheet, row, col int, c model.Cell) error {
	f := w.f
	name, err := excelize.CoordinatesToCellName(col+1, row+1)
	if err != nil {
		return err
	}
	switch c.Kind {
	case model.KindNumber:
		err = f.SetCellFloat(sh.Name, name, c.Num, -1, 64)
	case model.KindBool:
		err = f.SetCellBool(sh.Name, name, c.Num != 0)
	case model.KindString:
		if c.F != nil {
			// Text results are not cached (see FullCalcOnLoad).
			err = f.SetCellValue(sh.Name, name, nil)
		} else {
			err = f.SetCellStr(sh.Name, name, c.Str)
		}
	case model.KindError:
		if c.F == nil {
			// A literal error is stored as a formula that evaluates to it.
			err = f.SetCellFormula(sh.Name, name, c.Str)
		} else {
			err = f.SetCellValue(sh.Name, name, nil)
		}
	default:
		err = f.SetCellValue(sh.Name, name, nil)
	}
	if err != nil {
		return err
	}
	if c.F != nil {
		text := c.F.Text
		if node, perr := parser.Parse(text); perr == nil {
			text = ast.Format(ast.WithFilePrefixes(node))
		}
		if err := f.SetCellFormula(sh.Name, name, text); err != nil {
			return err
		}
	} else if c.Kind != model.KindError {
		if err := f.SetCellFormula(sh.Name, name, ""); err != nil {
			return err
		}
	}
	x, err := w.style(c.S)
	if err != nil {
		return err
	}
	return f.SetCellStyle(sh.Name, name, name, x)
}

// layout writes sizes, row/column styles and merges that differ from
// old (the layout in the file).
func (w *writer) layout(sh *model.Sheet, old model.Layout) error {
	f, l := w.f, sh.Layout
	for col := range union(old.Cols, l.Cols) {
		name, _ := excelize.ColumnNumberToName(col + 1)
		size, custom := l.Cols[col]
		width := l.DefaultCol
		if custom {
			width = size.Px
		}
		if err := f.SetColWidth(sh.Name, name, name, pxToWidth(width)); err != nil {
			return err
		}
		if err := f.SetColVisible(sh.Name, name, !size.Hidden); err != nil {
			return err
		}
	}
	for row := range union(old.Rows, l.Rows) {
		size, custom := l.Rows[row]
		height := l.DefaultRow
		if custom {
			height = size.Px
		}
		if err := f.SetRowHeight(sh.Name, row+1, pxToPt(height)); err != nil {
			return err
		}
		if err := f.SetRowVisible(sh.Name, row+1, !size.Hidden); err != nil {
			return err
		}
	}
	for col := range union(old.ColStyles, l.ColStyles) {
		x, err := w.style(l.ColStyles[col])
		if err != nil {
			return err
		}
		name, _ := excelize.ColumnNumberToName(col + 1)
		if err := f.SetColStyle(sh.Name, name, x); err != nil {
			return err
		}
	}
	for row := range union(old.RowStyles, l.RowStyles) {
		x, err := w.style(l.RowStyles[row])
		if err != nil {
			return err
		}
		if err := f.SetRowStyle(sh.Name, row+1, row+1, x); err != nil {
			return err
		}
	}
	cellName := func(r, c int) string { n, _ := excelize.CoordinatesToCellName(c+1, r+1); return n }
	isIn := func(list []model.Range, m model.Range) bool {
		for _, x := range list {
			if x == m {
				return true
			}
		}
		return false
	}
	for _, m := range old.Merges {
		if !isIn(l.Merges, m) {
			if err := f.UnmergeCell(sh.Name, cellName(m.R0, m.C0), cellName(m.R1, m.C1)); err != nil {
				return err
			}
		}
	}
	for _, m := range l.Merges {
		if !isIn(old.Merges, m) {
			if err := f.MergeCell(sh.Name, cellName(m.R0, m.C0), cellName(m.R1, m.C1)); err != nil {
				return err
			}
		}
	}
	for pos := range unionPos(old.Notes, l.Notes) {
		before, had := old.Notes[pos]
		after, has := l.Notes[pos]
		if had == has && before == after {
			continue
		}
		cell := cellName(pos[0], pos[1])
		if had {
			if err := f.DeleteComment(sh.Name, cell); err != nil {
				return err
			}
		}
		if has {
			author := after.Author
			if author == "" {
				author = "WW Excel"
			}
			if err := f.AddComment(sh.Name, excelize.Comment{
				Cell:   cell,
				Author: author,
				Paragraph: []excelize.RichTextRun{
					{Text: author + ":", Font: &excelize.Font{Bold: true}},
					{Text: "\n" + after.Text},
				},
			}); err != nil {
				return err
			}
		}
	}
	if err := w.tables(sh, old.Tables, l.Tables); err != nil {
		return err
	}
	if err := w.objects(sh, old.Objects, l.Objects); err != nil {
		return err
	}
	if err := w.pageSetup(sh, old.PageSetup, l.PageSetup); err != nil {
		return err
	}
	if !reflect.DeepEqual(l.Validations, old.Validations) {
		if err := writeValidations(f, sh.Name, l.Validations); err != nil {
			return err
		}
	}
	if !reflect.DeepEqual(l.CondFormats, old.CondFormats) {
		if err := writeCondFormats(f, sh.Name, l.CondFormats); err != nil {
			return err
		}
	}
	if l.FreezeRows != old.FreezeRows || l.FreezeCols != old.FreezeCols {
		panes := &excelize.Panes{}
		if l.FreezeRows > 0 || l.FreezeCols > 0 {
			active := "bottomRight"
			switch {
			case l.FreezeCols == 0:
				active = "bottomLeft"
			case l.FreezeRows == 0:
				active = "topRight"
			}
			panes = &excelize.Panes{Freeze: true, XSplit: l.FreezeCols, YSplit: l.FreezeRows,
				TopLeftCell: cellName(l.FreezeRows, l.FreezeCols), ActivePane: active}
		}
		if err := f.SetPanes(sh.Name, panes); err != nil {
			return err
		}
	}
	// Removing a filter is not written back: excelize cannot delete one.
	if fl := l.Filter; fl != nil && (old.Filter == nil || old.Filter.Range != fl.Range) {
		ref := cellName(fl.Range.R0, fl.Range.C0) + ":" + cellName(fl.Range.R1, fl.Range.C1)
		if err := f.AutoFilter(sh.Name, ref, nil); err != nil {
			return err
		}
	}
	return nil
}

func unionPos[V any](a, b map[[2]int]V) map[[2]int]bool {
	out := make(map[[2]int]bool, len(a)+len(b))
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

func union[V any](a, b map[int]V) map[int]bool {
	out := make(map[int]bool, len(a)+len(b))
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}
