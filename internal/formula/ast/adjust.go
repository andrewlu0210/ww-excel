package ast

import (
	"strings"

	"ww-excel/internal/model"
)

// Shift describes inserting (Count > 0) or deleting (Count < 0) rows or
// columns starting at index At on the named sheet.
type Shift struct {
	Sheet string
	Rows  bool // rows when true, columns otherwise
	At    int
	Count int
}

// Adjust rewrites the references in n that point at the shifted sheet,
// the way Excel does when rows or columns are inserted or deleted:
// references after the edit move, ranges spanning it grow or shrink,
// and references whose cells are deleted become #REF!. formulaSheet is
// the sheet the formula lives on (for references without a sheet).
func Adjust(n Node, formulaSheet string, s Shift) (Node, bool) {
	changed := false
	var rewrite func(Node) Node
	rewrite = func(n Node) Node {
		switch x := n.(type) {
		case Ref:
			sheet := x.Sheet
			if sheet == "" {
				sheet = formulaSheet
			}
			if !strings.EqualFold(sheet, s.Sheet) {
				return x
			}
			a, ok, moved := adjustArea(x.Area, s)
			if !ok {
				changed = true
				return Error{Code: model.ErrRef}
			}
			if moved {
				changed = true
				x.Area = a
			}
			return x
		case Paren:
			return Paren{X: rewrite(x.X)}
		case Unary:
			return Unary{Op: x.Op, X: rewrite(x.X)}
		case Percent:
			return Percent{X: rewrite(x.X)}
		case Binary:
			return Binary{Op: x.Op, L: rewrite(x.L), R: rewrite(x.R)}
		case Call:
			args := make([]Node, len(x.Args))
			for i, a := range x.Args {
				args[i] = rewrite(a)
			}
			return Call{Name: x.Name, Args: args}
		default:
			return n
		}
	}
	out := rewrite(n)
	return out, changed
}

// adjustArea returns the new area, false if it was deleted entirely,
// and whether it moved.
func adjustArea(a Area, s Shift) (Area, bool, bool) {
	// Whole-column areas ignore row shifts and vice versa.
	if s.Rows && a.Kind == ColArea || !s.Rows && a.Kind == RowArea {
		return a, true, false
	}
	limit := model.MaxCols
	lo, hi := &a.C0, &a.C1
	if s.Rows {
		limit = model.MaxRows
		lo, hi = &a.R0, &a.R1
	}
	oldLo, oldHi := *lo, *hi
	newLo, newHi, ok := shiftSpan(oldLo, oldHi, s.At, s.Count, limit)
	if !ok {
		return a, false, false
	}
	*lo, *hi = newLo, newHi
	return a, true, newLo != oldLo || newHi != oldHi
}

func shiftSpan(lo, hi, at, count, limit int) (int, int, bool) {
	if count > 0 {
		if lo >= at {
			lo += count
		}
		if hi >= at {
			hi += count
		}
		if lo >= limit {
			return 0, 0, false
		}
		return lo, min(hi, limit-1), true
	}
	n := -count
	end := at + n - 1
	switch {
	case hi < at:
		return lo, hi, true
	case lo > end:
		return lo - n, hi - n, true
	}
	newLo := lo
	if lo >= at {
		newLo = at
	}
	newHi := hi - n
	if hi <= end {
		newHi = at - 1
	}
	if newHi < newLo {
		return 0, 0, false
	}
	return newLo, newHi, true
}

// Translate moves the relative ($-less) parts of references by (dr, dc),
// as when a formula is copied or filled to another cell. References that
// would leave the grid become #REF!.
func Translate(n Node, dr, dc int) Node {
	if dr == 0 && dc == 0 {
		return n
	}
	var rewrite func(Node) Node
	rewrite = func(n Node) Node {
		switch x := n.(type) {
		case Ref:
			a, ok := translateArea(x.Area, dr, dc)
			if !ok {
				return Error{Code: model.ErrRef}
			}
			x.Area = a
			return x
		case Paren:
			return Paren{X: rewrite(x.X)}
		case Unary:
			return Unary{Op: x.Op, X: rewrite(x.X)}
		case Percent:
			return Percent{X: rewrite(x.X)}
		case Binary:
			return Binary{Op: x.Op, L: rewrite(x.L), R: rewrite(x.R)}
		case Call:
			args := make([]Node, len(x.Args))
			for i, a := range x.Args {
				args[i] = rewrite(a)
			}
			return Call{Name: x.Name, Args: args}
		default:
			return n
		}
	}
	return rewrite(n)
}

func translateArea(a Area, dr, dc int) (Area, bool) {
	move := func(v int, abs bool, d int) int {
		if abs {
			return v
		}
		return v + d
	}
	if a.Kind != ColArea {
		a.R0, a.R1 = move(a.R0, a.AbsR0, dr), move(a.R1, a.AbsR1, dr)
	}
	if a.Kind != RowArea {
		a.C0, a.C1 = move(a.C0, a.AbsC0, dc), move(a.C1, a.AbsC1, dc)
	}
	if a.R0 < 0 || a.C0 < 0 || a.R1 < 0 || a.C1 < 0 ||
		a.R0 >= model.MaxRows || a.R1 >= model.MaxRows || a.C0 >= model.MaxCols || a.C1 >= model.MaxCols {
		return a, false
	}
	// A mixed range such as A1:$A$5 can flip when moved. Excel swaps the
	// whole corner references, giving $A$5:A11.
	if a.R0 > a.R1 {
		a.R0, a.R1, a.AbsR0, a.AbsR1 = a.R1, a.R0, a.AbsR1, a.AbsR0
		a.C0, a.C1, a.AbsC0, a.AbsC1 = a.C1, a.C0, a.AbsC1, a.AbsC0
	}
	if a.C0 > a.C1 {
		a.C0, a.C1, a.AbsC0, a.AbsC1 = a.C1, a.C0, a.AbsC1, a.AbsC0
	}
	return a, true
}
