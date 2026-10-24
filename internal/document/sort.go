package document

import (
	"errors"
	"sort"
	"strings"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
)

// SortKey sorts by one column (an absolute column index).
type SortKey struct {
	Col  int
	Desc bool
}

// CurrentRegion is the block of data around (row, col) bounded by blank
// rows and columns, like Excel's ⌃* selection.
func (d *Document) CurrentRegion(sheet, row, col int) model.Range {
	rg := model.Range{R0: row, C0: col, R1: row, C1: col}
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return rg
	}
	sh := d.wb.Sheets[sheet]
	has := func(r model.Range) bool {
		r = r.Clamp()
		if r.R0 > r.R1 || r.C0 > r.C1 {
			return false
		}
		if r.CellCount() <= 256 {
			for row := r.R0; row <= r.R1; row++ {
				for col := r.C0; col <= r.C1; col++ {
					if sh.Get(row, col).HasContent() {
						return true
					}
				}
			}
			return false
		}
		found := false
		sh.EachInRange(r, func(_, _ int, c model.Cell) {
			found = found || c.HasContent()
		})
		return found
	}
	for changed := true; changed; {
		changed = false
		for rg.R0 > 0 && has(model.Range{R0: rg.R0 - 1, C0: rg.C0 - 1, R1: rg.R0 - 1, C1: rg.C1 + 1}) {
			rg.R0--
			changed = true
		}
		for rg.R1 < model.MaxRows-1 && has(model.Range{R0: rg.R1 + 1, C0: rg.C0 - 1, R1: rg.R1 + 1, C1: rg.C1 + 1}) {
			rg.R1++
			changed = true
		}
		for rg.C0 > 0 && has(model.Range{R0: rg.R0 - 1, C0: rg.C0 - 1, R1: rg.R1 + 1, C1: rg.C0 - 1}) {
			rg.C0--
			changed = true
		}
		for rg.C1 < model.MaxCols-1 && has(model.Range{R0: rg.R0 - 1, C0: rg.C1 + 1, R1: rg.R1 + 1, C1: rg.C1 + 1}) {
			rg.C1++
			changed = true
		}
	}
	return rg
}

// SortRange returns the range a sort applies to: the selection when it
// is more than one cell, else the current region (ok false when that is
// empty), and whether its first row looks like a header.
func (d *Document) SortRange(sheet int, sel model.Range) (rg model.Range, header, ok bool) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return sel, false, false
	}
	sh := d.wb.Sheets[sheet]
	rg = sel.Clamp()
	if rg.R0 == rg.R1 && rg.C0 == rg.C1 {
		rg = d.CurrentRegion(sheet, rg.R0, rg.C0)
		if rg.R0 == rg.R1 && rg.C0 == rg.C1 && !sh.Get(rg.R0, rg.C0).HasContent() {
			return rg, false, false
		}
	}
	// Whole columns: stop at the used rows.
	if used, _ := sh.UsedRange(); rg.R1 >= used {
		rg.R1 = max(rg.R0, used-1)
	}
	return rg, d.looksLikeHeader(sh, rg), true
}

// looksLikeHeader guesses, like Excel, that the first row is a header
// when it is all text (or bold) while the row below is not.
func (d *Document) looksLikeHeader(sh *model.Sheet, rg model.Range) bool {
	if rg.R1 <= rg.R0 {
		return false
	}
	st := d.wb.StyleTable()
	allText, bold, below, any := true, true, false, false
	for c := rg.C0; c <= rg.C1; c++ {
		h := sh.Get(rg.R0, c)
		if h.HasContent() {
			any = true
			if h.F != nil || h.Kind != model.KindString {
				allText = false
			}
		}
		if !st.Get(sh.EffectiveStyle(rg.R0, c)).Font.Bold {
			bold = false
		}
		if n := sh.Get(rg.R0+1, c); n.HasContent() && (n.Kind != model.KindString || n.F != nil) {
			below = true
		}
	}
	return any && (allText && below || bold && !d.rowBold(sh, rg.R0+1, rg))
}

func (d *Document) rowBold(sh *model.Sheet, row int, rg model.Range) bool {
	st := d.wb.StyleTable()
	for c := rg.C0; c <= rg.C1; c++ {
		if !st.Get(sh.EffectiveStyle(row, c)).Font.Bold {
			return false
		}
	}
	return true
}

// SortCommand sorts the rows of rg (below the header row when header)
// by keys. Blanks sort last in both directions; numbers come before
// text, then logicals and errors, as in Excel. Formulas move with their
// row and their relative references are adjusted.
func (d *Document) SortCommand(sheet int, rg model.Range, keys []SortKey, header bool) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) || len(keys) == 0 {
		return nil, errors.New("無效的排序")
	}
	sh := d.wb.Sheets[sheet]
	rg = rg.Clamp()
	if header {
		rg.R0++
	}
	if rg.R1 <= rg.R0 {
		return nil, nil
	}
	if rg.CellCount() > command.MaxFillCells {
		return nil, errors.New("排序範圍太大")
	}
	for _, m := range sh.Layout.Merges {
		if m.Overlaps(rg) {
			return nil, errors.New("無法排序：範圍內有合併儲存格")
		}
	}
	for _, k := range keys {
		if k.Col < rg.C0 || k.Col > rg.C1 {
			return nil, errors.New("排序欄不在範圍內")
		}
	}

	type row struct {
		src   int
		cells map[int]model.Cell
	}
	rows := make([]row, rg.R1-rg.R0+1)
	for i := range rows {
		rows[i] = row{src: rg.R0 + i, cells: map[int]model.Cell{}}
	}
	sh.EachInRange(rg, func(r, c int, cell model.Cell) {
		rows[r-rg.R0].cells[c] = cell
	})
	sort.SliceStable(rows, func(i, j int) bool {
		for _, k := range keys {
			a, b := rows[i].cells[k.Col], rows[j].cells[k.Col]
			if cmp := compareSort(a, b, k.Desc); cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})

	var changes []command.CellChange
	for i, r := range rows {
		dst := rg.R0 + i
		if r.src == dst {
			continue
		}
		for c := rg.C0; c <= rg.C1; c++ {
			cell, ok := r.cells[c]
			old := sh.Get(dst, c)
			if !ok {
				if old.IsEmpty() {
					continue
				}
				changes = append(changes, command.CellChange{Row: dst, Col: c, After: model.Cell{}})
				continue
			}
			if cell.F != nil {
				node := ast.Translate(d.calc.Parsed(cell.F, sheet), dst-r.src, 0)
				f := d.calc.CompileNode(node, sheet)
				cell.F = f
			}
			changes = append(changes, command.CellChange{Row: dst, Col: c, After: cell})
		}
	}
	if len(changes) == 0 {
		return nil, nil
	}
	return &command.SetCells{Sheet: sheet, Name: "排序", Changes: changes, Area: &rg}, nil
}

// sortRank orders kinds: numbers, text, logicals, errors; blanks last.
func sortRank(c model.Cell) int {
	switch c.Kind {
	case model.KindNumber:
		return 0
	case model.KindString:
		if c.Str == "" {
			return 4
		}
		return 1
	case model.KindBool:
		return 2
	case model.KindError:
		return 3
	}
	return 4
}

func compareSort(a, b model.Cell, desc bool) int {
	ra, rb := sortRank(a), sortRank(b)
	if ra == 4 || rb == 4 {
		return ra - rb // blanks last either way
	}
	cmp := 0
	switch {
	case ra != rb:
		cmp = ra - rb
	case ra == 0:
		cmp = cmpFloat(a.Num, b.Num)
	case ra == 1:
		cmp = strings.Compare(strings.ToLower(a.Str), strings.ToLower(b.Str))
	case ra == 2:
		cmp = cmpFloat(a.Num, b.Num)
	}
	if desc {
		return -cmp
	}
	return cmp
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
