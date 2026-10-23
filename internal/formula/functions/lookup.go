package functions

import (
	"ww-excel/internal/formula/eval"
)

func init() {
	reg(catLookup, "VLOOKUP", 3, 4, "lookup_value, table_array, col_index_num, [range_lookup]", "在表格第一欄尋找值，傳回同列指定欄的值", func(a *eval.Args) eval.Value {
		return tableLookup(a, false)
	})
	reg(catLookup, "HLOOKUP", 3, 4, "lookup_value, table_array, row_index_num, [range_lookup]", "在表格第一列尋找值，傳回同欄指定列的值", func(a *eval.Args) eval.Value {
		return tableLookup(a, true)
	})
	reg(catLookup, "MATCH", 2, 3, "lookup_value, lookup_array, [match_type]", "傳回值在範圍中的相對位置", func(a *eval.Args) eval.Value {
		v := a.Scalar(0)
		if v.IsErr() {
			return v
		}
		g, err := a.Grid(1)
		if err.IsErr() {
			return err
		}
		mode, err := a.Int(2, 1)
		if err.IsErr() {
			return err
		}
		line, ok := lineOf(g)
		if !ok {
			return errNA
		}
		var idx int
		switch {
		case mode == 0:
			idx = findExact(line, v, 1)
		case mode > 0:
			idx = findSorted(line, v, true)
		default:
			idx = findSorted(line, v, false)
		}
		if idx < 0 {
			return errNA
		}
		return eval.Number(float64(idx + 1))
	})
	reg(catLookup, "INDEX", 2, 3, "array, row_num, [column_num]", "傳回範圍中指定列與欄交會處的值", func(a *eval.Args) eval.Value {
		g, err := a.Grid(0)
		if err.IsErr() {
			return err
		}
		row, err := a.Int(1, 1)
		if err.IsErr() {
			return err
		}
		col, err := a.Int(2, 1)
		if err.IsErr() {
			return err
		}
		// A single row with one index picks a column: INDEX(A1:C1, 2).
		if g.Rows() == 1 && !a.Has(2) && a.Len() == 2 {
			row, col = 1, row
		}
		if row == 0 || col == 0 {
			return errValue // whole row/column results need dynamic arrays
		}
		if row < 0 || col < 0 {
			return errValue
		}
		rows, cols := indexDims(a, g)
		if row > rows || col > cols {
			return errRef
		}
		if row > g.Rows() || col > g.Cols() {
			return eval.Empty // inside the reference but beyond the used range
		}
		return g.At(row-1, col-1)
	})
	reg(catLookup, "XLOOKUP", 3, 6, "lookup_value, lookup_array, return_array, [if_not_found], [match_mode], [search_mode]", "在範圍中尋找值並傳回另一範圍中對應的值", xlookup)
	reg(catLookup, "CHOOSE", 2, variadic, "index_num, value1, [value2], ...", "依索引從清單中選擇值", func(a *eval.Args) eval.Value {
		i, err := a.Int(0, 0)
		if err.IsErr() {
			return err
		}
		if i < 1 || i >= a.Len() {
			return errValue
		}
		return a.Raw(i)
	})
	reg(catLookup, "ROW", 0, 1, "[reference]", "傳回參照的列號", func(a *eval.Args) eval.Value {
		if !a.Has(0) {
			return eval.Number(float64(a.Ev().Row + 1))
		}
		v := a.Raw(0)
		if v.Kind != eval.KRef {
			return errValue
		}
		return eval.Number(float64(v.Ref.R0 + 1))
	})
	reg(catLookup, "COLUMN", 0, 1, "[reference]", "傳回參照的欄號", func(a *eval.Args) eval.Value {
		if !a.Has(0) {
			return eval.Number(float64(a.Ev().Col + 1))
		}
		v := a.Raw(0)
		if v.Kind != eval.KRef {
			return errValue
		}
		return eval.Number(float64(v.Ref.C0 + 1))
	})
	reg(catLookup, "ROWS", 1, 1, "array", "傳回參照或陣列的列數", func(a *eval.Args) eval.Value {
		v := a.Raw(0)
		switch v.Kind {
		case eval.KRef:
			return eval.Number(float64(v.Ref.Rows()))
		case eval.KArray:
			return eval.Number(float64(v.Arr.Rows))
		case eval.KError:
			return v
		}
		return eval.Number(1)
	})
	reg(catLookup, "COLUMNS", 1, 1, "array", "傳回參照或陣列的欄數", func(a *eval.Args) eval.Value {
		v := a.Raw(0)
		switch v.Kind {
		case eval.KRef:
			return eval.Number(float64(v.Ref.Cols()))
		case eval.KArray:
			return eval.Number(float64(v.Arr.Cols))
		case eval.KError:
			return v
		}
		return eval.Number(1)
	})
}

// indexDims is the full size of INDEX's first argument (references are
// not limited to the used range).
func indexDims(a *eval.Args, g eval.Grid) (int, int) {
	if v := a.Raw(0); v.Kind == eval.KRef {
		return v.Ref.Rows(), v.Ref.Cols()
	}
	return g.Rows(), g.Cols()
}

// line is a 1-D view over a grid row or column.
type line struct {
	g        eval.Grid
	vertical bool
}

func (l line) Len() int {
	if l.vertical {
		return l.g.Rows()
	}
	return l.g.Cols()
}

func (l line) At(i int) eval.Value {
	if l.vertical {
		return l.g.At(i, 0)
	}
	return l.g.At(0, i)
}

func lineOf(g eval.Grid) (line, bool) {
	switch {
	case g.Cols() == 1:
		return line{g, true}, true
	case g.Rows() == 1:
		return line{g, false}, true
	}
	return line{}, false
}

// exactEqual compares for exact-match lookups: same type, text
// case-insensitive with wildcards when allowed.
func exactEqual(cell, v eval.Value, wildcards bool) bool {
	if cell.Kind == eval.KEmpty || v.Kind == eval.KEmpty {
		return false
	}
	if v.Kind == eval.KString {
		if cell.Kind != eval.KString {
			return false
		}
		if wildcards {
			return eval.WildMatch(v.Str, cell.Str)
		}
		return eval.Compare(cell, v) == 0
	}
	return cell.Kind == v.Kind && eval.Compare(cell, v) == 0
}

// findExact returns the first (dir 1) or last (dir -1) exact match.
func findExact(l line, v eval.Value, dir int) int {
	n := l.Len()
	for k := 0; k < n; k++ {
		i := k
		if dir < 0 {
			i = n - 1 - k
		}
		if exactEqual(l.At(i), v, true) {
			return i
		}
	}
	return -1
}

func comparable(cell, v eval.Value) bool {
	if cell.Kind == eval.KEmpty || cell.Kind == eval.KError {
		return false
	}
	isText := func(k eval.Kind) bool { return k == eval.KString }
	return isText(cell.Kind) == isText(v.Kind) && (cell.Kind == eval.KBool) == (v.Kind == eval.KBool)
}

// findSorted is approximate matching on sorted data: ascending finds
// the last value <= v (stopping at the first larger one); descending
// finds the last value >= v.
func findSorted(l line, v eval.Value, ascending bool) int {
	best := -1
	for i := 0; i < l.Len(); i++ {
		cell := l.At(i)
		if !comparable(cell, v) {
			continue
		}
		c := eval.Compare(cell, v)
		if !ascending {
			c = -c
		}
		if c > 0 {
			break
		}
		best = i
	}
	return best
}

func tableLookup(a *eval.Args, horizontal bool) eval.Value {
	v := a.Scalar(0)
	if v.IsErr() {
		return v
	}
	g, err := a.Grid(1)
	if err.IsErr() {
		return err
	}
	idx, err := a.Int(2, 0)
	if err.IsErr() {
		return err
	}
	approx, err := a.Bool(3, true)
	if err.IsErr() {
		return err
	}
	size := g.Cols()
	if horizontal {
		size = g.Rows()
	}
	if idx < 1 {
		return errValue
	}
	if r, ok := g.IsRef(); ok {
		// Compare against the full reference width, not the used range.
		if horizontal {
			size = max(size, r.Rows())
		} else {
			size = max(size, r.Cols())
		}
	}
	if idx > size {
		return errRef
	}

	key := line{g, !horizontal}
	var found int
	if approx {
		found = findSorted(key, v, true)
	} else {
		found = findExact(key, v, 1)
	}
	if found < 0 {
		return errNA
	}
	if horizontal {
		if idx > g.Rows() {
			return eval.Empty
		}
		return g.At(idx-1, found)
	}
	if idx > g.Cols() {
		return eval.Empty
	}
	return g.At(found, idx-1)
}

func xlookup(a *eval.Args) eval.Value {
	v := a.Scalar(0)
	if v.IsErr() {
		return v
	}
	look, err := a.Grid(1)
	if err.IsErr() {
		return err
	}
	ret, err := a.Grid(2)
	if err.IsErr() {
		return err
	}
	mode, err := a.Int(4, 0)
	if err.IsErr() {
		return err
	}
	search, err := a.Int(5, 1)
	if err.IsErr() {
		return err
	}
	l, ok := lineOf(look)
	if !ok {
		return errValue
	}
	dir := 1
	if search < 0 {
		dir = -1
	}

	idx := -1
	switch mode {
	case 0, 2:
		n := l.Len()
		for k := 0; k < n && idx < 0; k++ {
			i := k
			if dir < 0 {
				i = n - 1 - k
			}
			if exactEqual(l.At(i), v, mode == 2) {
				idx = i
			}
		}
	case -1, 1:
		// Exact match, else the next smaller (-1) or larger (1) item.
		for i := 0; i < l.Len(); i++ {
			cell := l.At(i)
			if !comparable(cell, v) {
				continue
			}
			c := eval.Compare(cell, v)
			if c == 0 {
				idx = i
				break
			}
			if mode == -1 && c < 0 && (idx < 0 || eval.Compare(cell, l.At(idx)) > 0) ||
				mode == 1 && c > 0 && (idx < 0 || eval.Compare(cell, l.At(idx)) < 0) {
				idx = i
			}
		}
	default:
		return errValue
	}

	if idx < 0 {
		if a.Has(3) {
			return a.Raw(3)
		}
		return errNA
	}
	// Return the matching row (or column) of return_array; without
	// dynamic arrays only its first element is shown.
	if l.vertical {
		if idx >= ret.Rows() {
			return eval.Empty
		}
		return ret.At(idx, 0)
	}
	if idx >= ret.Cols() {
		return eval.Empty
	}
	return ret.At(0, idx)
}
