package document

import (
	"errors"
	"slices"
	"sort"

	"ww-excel/internal/command"
	"ww-excel/internal/model"
)

func (d *Document) layoutCommand(sheet int, name string, area *model.Range, edit func(l *model.Layout)) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	after := d.wb.Sheets[sheet].Layout.Clone()
	edit(&after)
	return &command.Layout{Sheet: sheet, Name: name, After: after, Area: area}, nil
}

// FreezeCommand freezes the first rows and cols (0, 0 unfreezes).
func (d *Document) FreezeCommand(sheet, rows, cols int) (command.Command, error) {
	if rows < 0 || cols < 0 || rows >= model.MaxRows || cols >= model.MaxCols {
		return nil, errors.New("無效的凍結位置")
	}
	name := "凍結窗格"
	if rows == 0 && cols == 0 {
		name = "取消凍結窗格"
	}
	return d.layoutCommand(sheet, name, nil, func(l *model.Layout) {
		l.FreezeRows, l.FreezeCols = rows, cols
	})
}

// HideCommand hides or unhides rows (rows = true) or columns. Unhiding
// a span shows every hidden row or column in it.
func (d *Document) HideCommand(sheet int, rows bool, lo, hi int, hidden bool) (command.Command, error) {
	if lo > hi {
		lo, hi = hi, lo
	}
	limit := model.MaxCols
	if rows {
		limit = model.MaxRows
	}
	lo, hi = max(lo, 0), min(hi, limit-1)
	what := map[bool]string{true: "列", false: "欄"}[rows]
	name := "隱藏" + what
	if !hidden {
		name = "取消隱藏" + what
	}
	if hidden && lo == 0 && hi == limit-1 {
		return nil, errors.New("無法隱藏所有的" + what)
	}
	return d.layoutCommand(sheet, name, nil, func(l *model.Layout) {
		sizes, def := l.Cols, l.DefaultCol
		if rows {
			sizes, def = l.Rows, l.DefaultRow
		}
		if !hidden {
			for i, s := range sizes {
				if i >= lo && i <= hi && s.Hidden {
					s.Hidden = false
					if s.Px == def || s.Px <= 0 {
						delete(sizes, i)
					} else {
						sizes[i] = s
					}
				}
			}
			return
		}
		for i := lo; i <= hi; i++ {
			s, ok := sizes[i]
			if !ok {
				s = model.Size{Px: def}
			}
			s.Hidden = true
			sizes[i] = s
		}
	})
}

// ToggleFilterCommand turns the AutoFilter off, or on for the range
// around the selection (its first row holds the buttons).
func (d *Document) ToggleFilterCommand(sheet int, sel model.Range) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	sh := d.wb.Sheets[sheet]
	if f := sh.Layout.Filter; f != nil {
		return d.layoutCommand(sheet, "移除篩選", nil, func(l *model.Layout) {
			showRows(l, f.Range.R0+1, f.Range.R1)
			l.Filter = nil
		})
	}
	rg, _, ok := d.SortRange(sheet, sel)
	if !ok {
		return nil, errors.New("請選取資料範圍中的儲存格")
	}
	return d.layoutCommand(sheet, "篩選", nil, func(l *model.Layout) {
		l.Filter = &model.AutoFilter{Range: rg, Cols: map[int][]string{}}
	})
}

func showRows(l *model.Layout, lo, hi int) {
	for i, s := range l.Rows {
		if i >= lo && i <= hi && s.Hidden {
			if s.Px == l.DefaultRow {
				delete(l.Rows, i)
			} else {
				s.Hidden = false
				l.Rows[i] = s
			}
		}
	}
}

// FilterValue is one entry in a filter dropdown.
type FilterValue struct {
	Text  string
	Shown bool
}

// FilterForColumn also exposes a table's filter before criteria are applied.
func (d *Document) FilterForColumn(sheet, col int) *model.AutoFilter {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil
	}
	l := &d.wb.Sheets[sheet].Layout
	if f := l.Filter; f != nil && col >= f.Range.C0 && col <= f.Range.C1 {
		return f
	}
	for _, t := range l.Tables {
		if !t.NoFilter && col >= t.Range.C0 && col <= t.Range.C1 {
			rg := t.Range
			rg.R1 = t.DataRows().R1
			return &model.AutoFilter{Range: rg, Cols: map[int][]string{}}
		}
	}
	return nil
}

// FilterValues lists the distinct display values of a filter column
// (data rows only), sorted like an ascending sort; "" stands for blanks.
func (d *Document) FilterValues(sheet, col int, display DisplayFunc) []FilterValue {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil
	}
	sh := d.wb.Sheets[sheet]
	f := d.FilterForColumn(sheet, col)
	if f == nil || col < f.Range.C0 || col > f.Range.C1 {
		return nil
	}
	type entry struct {
		text string
		cell model.Cell
	}
	seen := map[string]bool{}
	var entries []entry
	blank := false
	for r := f.Range.R0 + 1; r <= f.Range.R1; r++ {
		c := sh.Get(r, col)
		t := ""
		if c.HasContent() {
			t = display(sh, r, col, c)
		}
		if t == "" {
			blank = true
			continue
		}
		if !seen[t] {
			seen[t] = true
			entries = append(entries, entry{t, c.Value()})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return compareSort(entries[i].cell, entries[j].cell, false) < 0 })
	allowed, filtered := f.Cols[col]
	out := make([]FilterValue, 0, len(entries)+1)
	for _, e := range entries {
		out = append(out, FilterValue{Text: e.text, Shown: !filtered || slices.Contains(allowed, e.text)})
	}
	if blank {
		out = append(out, FilterValue{Text: "", Shown: !filtered || slices.Contains(allowed, "")})
	}
	return out
}

// FilterCommand sets the values a filter column shows (nil = all) and
// hides the data rows that fail any column's criteria.
func (d *Document) FilterCommand(sheet, col int, shown []string, display DisplayFunc) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	sh := d.wb.Sheets[sheet]
	f := d.FilterForColumn(sheet, col)
	if f == nil || col < f.Range.C0 || col > f.Range.C1 {
		return nil, errors.New("這一欄沒有篩選")
	}
	nf := f.Clone()
	if shown == nil {
		delete(nf.Cols, col)
	} else {
		nf.Cols[col] = append([]string{}, shown...)
	}
	return d.layoutCommand(sheet, "篩選", nil, func(l *model.Layout) {
		if l.Filter != nil && l.Filter.Range != nf.Range {
			showRows(l, l.Filter.Range.R0+1, l.Filter.Range.R1)
		}
		l.Filter = nf
		d.applyFilter(sh, l, display)
	})
}

// applyFilter shows or hides every data row of the filter range.
func (d *Document) applyFilter(sh *model.Sheet, l *model.Layout, display DisplayFunc) {
	f := l.Filter
	sets := map[int]map[string]bool{}
	for c, vals := range f.Cols {
		set := make(map[string]bool, len(vals))
		for _, v := range vals {
			set[v] = true
		}
		sets[c] = set
	}
	showRows(l, f.Range.R0+1, f.Range.R1)
	if len(sets) == 0 {
		return
	}
	for r := f.Range.R0 + 1; r <= f.Range.R1; r++ {
		for c, set := range sets {
			cell := sh.Get(r, c)
			t := ""
			if cell.HasContent() {
				t = display(sh, r, c, cell)
			}
			if !set[t] {
				s, ok := l.Rows[r]
				if !ok {
					s = model.Size{Px: l.DefaultRow}
				}
				s.Hidden = true
				l.Rows[r] = s
				break
			}
		}
	}
}

// NoteCommand sets the note of a cell; empty text deletes it.
func (d *Document) NoteCommand(sheet, row, col int, text, author string) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) || row < 0 || row >= model.MaxRows || col < 0 || col >= model.MaxCols {
		return nil, errors.New("無效的儲存格")
	}
	pos := [2]int{row, col}
	old, had := d.wb.Sheets[sheet].Layout.Notes[pos]
	name := "插入註解"
	switch {
	case text == "" && !had:
		return nil, nil
	case text == "":
		name = "刪除註解"
	case had:
		if old.Text == text {
			return nil, nil
		}
		name = "編輯註解"
		author = old.Author
	}
	area := model.Range{R0: row, C0: col, R1: row, C1: col}
	return d.layoutCommand(sheet, name, &area, func(l *model.Layout) {
		if l.Notes == nil {
			l.Notes = map[[2]int]model.Note{}
		}
		if text == "" {
			delete(l.Notes, pos)
		} else {
			l.Notes[pos] = model.Note{Author: author, Text: text}
		}
	})
}
