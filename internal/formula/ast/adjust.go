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
	out := mapRefs(n, func(x Ref) Node {
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
	})
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
	newLo, newHi, ok := model.ShiftSpan(oldLo, oldHi, s.At, s.Count, limit)
	if !ok {
		return a, false, false
	}
	*lo, *hi = newLo, newHi
	return a, true, newLo != oldLo || newHi != oldHi
}

// Translate moves the relative ($-less) parts of references by (dr, dc),
// as when a formula is copied or filled to another cell. References that
// would leave the grid become #REF!.
func Translate(n Node, dr, dc int) Node {
	if dr == 0 && dc == 0 {
		return n
	}
	return mapRefs(n, func(x Ref) Node {
		a, ok := translateArea(x.Area, dr, dc)
		if !ok {
			return Error{Code: model.ErrRef}
		}
		x.Area = a
		return x
	})
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

// RenameSheet rewrites references to sheet old (case-insensitive) to
// use name; returns whether anything changed.
func RenameSheet(n Node, old, name string) (Node, bool) {
	changed := false
	out := mapRefs(n, func(r Ref) Node {
		if r.Sheet != "" && strings.EqualFold(r.Sheet, old) {
			r.Sheet = name
			changed = true
		}
		return r
	})
	return out, changed
}

// RemoveSheet turns references to a deleted sheet into #REF!.
func RemoveSheet(n Node, name string) (Node, bool) {
	changed := false
	out := mapRefs(n, func(r Ref) Node {
		if r.Sheet != "" && strings.EqualFold(r.Sheet, name) {
			changed = true
			return Error{Code: model.ErrRef}
		}
		return r
	})
	return out, changed
}

// Functions added after Excel 2007 are stored with an _xlfn. prefix in
// xlsx files; without it Excel shows #NAME?.
var futureFunctions = map[string]bool{
	"IFS": true, "SWITCH": true, "XOR": true, "IFNA": true, "CONCAT": true,
	"TEXTJOIN": true, "XLOOKUP": true, "DAYS": true, "MAXIFS": true, "MINIFS": true,
}

// WithFilePrefixes adds _xlfn. to newer functions for writing xlsx.
func WithFilePrefixes(n Node) Node {
	return mapCalls(n, func(c Call) Call {
		if futureFunctions[c.Name] {
			c.Name = "_xlfn." + c.Name
		}
		return c
	})
}

func mapRefs(n Node, fn func(Ref) Node) Node {
	return rewriteTree(n, fn, nil)
}

// MapTableRefs replaces structured references; changed reports whether
// fn returned anything different.
func MapTableRefs(n Node, fn func(TableRef) Node) (Node, bool) {
	changed := false
	var rewrite func(Node) Node
	rewrite = func(n Node) Node {
		switch x := n.(type) {
		case TableRef:
			out := fn(x)
			if out != Node(x) {
				changed = true
			}
			return out
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
		}
		return n
	}
	out := rewrite(n)
	return out, changed
}

func mapCalls(n Node, fn func(Call) Call) Node {
	return rewriteTree(n, nil, fn)
}

func rewriteTree(n Node, refFn func(Ref) Node, callFn func(Call) Call) Node {
	var rewrite func(Node) Node
	rewrite = func(n Node) Node {
		switch x := n.(type) {
		case Ref:
			if refFn != nil {
				return refFn(x)
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
			c := Call{Name: x.Name, Args: args}
			if callFn != nil {
				c = callFn(c)
			}
			return c
		default:
			return n
		}
	}
	return rewrite(n)
}

// Move rewrites references that lie entirely inside src on sheet srcSheet
// to point (dr, dc) away on dstSheet, as Excel does for formulas that
// refer to cells that were cut and pasted. Partially covered references
// are left alone. formulaSheet is the sheet the formula lives on.
func Move(n Node, formulaSheet, srcSheet string, src model.Range, dstSheet string, dr, dc int) (Node, bool) {
	changed := false
	out := mapRefs(n, func(x Ref) Node {
		sheet := x.Sheet
		if sheet == "" {
			sheet = formulaSheet
		}
		a := x.Area
		if !strings.EqualFold(sheet, srcSheet) || a.Kind != CellArea ||
			a.R0 < src.R0 || a.R1 > src.R1 || a.C0 < src.C0 || a.C1 > src.C1 {
			return x
		}
		a.R0, a.R1, a.C0, a.C1 = a.R0+dr, a.R1+dr, a.C0+dc, a.C1+dc
		x.Area = a
		if !strings.EqualFold(dstSheet, formulaSheet) || x.Sheet != "" {
			x.Sheet = dstSheet
		}
		if strings.EqualFold(dstSheet, formulaSheet) && x.Sheet != "" && strings.EqualFold(x.Sheet, formulaSheet) && sheet == formulaSheet {
			x.Sheet = ""
		}
		changed = true
		return x
	})
	return out, changed
}
