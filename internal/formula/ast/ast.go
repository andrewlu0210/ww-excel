// Package ast defines formula syntax trees. Nodes are immutable; edits
// such as reference adjustment return new trees.
package ast

import "ww-excel/internal/model"

type Node interface{ node() }

type (
	Number struct{ V float64 }
	String struct{ V string }
	Bool   struct{ V bool }
	// Error is an error literal such as #N/A, or a reference that became
	// invalid (#REF!).
	Error struct{ Code string }
	// Missing is an omitted argument: IF(A1,,1).
	Missing struct{}
	// Paren keeps user parentheses so formulas print as typed.
	Paren struct{ X Node }
	Unary struct {
		Op byte // '-' or '+'
		X  Node
	}
	Percent struct{ X Node }
	Binary  struct {
		Op   string // + - * / ^ & = <> < > <= >=
		L, R Node
	}
	Call struct {
		Name string // upper case
		Args []Node
	}
	// Array is an array constant {1,2;3,4}; elements are literals.
	Array struct{ Rows [][]Node }
	// Name is an identifier that is not a function or reference, e.g. a
	// defined name (v0.6). Evaluates to #NAME? for now.
	Name struct{ Name string }
	Ref  struct {
		Sheet string // "" means the formula's own sheet
		Area  Area
	}
)

func (Number) node()  {}
func (String) node()  {}
func (Bool) node()    {}
func (Error) node()   {}
func (Missing) node() {}
func (Paren) node()   {}
func (Unary) node()   {}
func (Percent) node() {}
func (Binary) node()  {}
func (Call) node()    {}
func (Array) node()   {}
func (Name) node()    {}
func (Ref) node()     {}

type AreaKind uint8

const (
	CellArea AreaKind = iota // A1 or A1:B2
	ColArea                  // A:B
	RowArea                  // 1:2
)

// Area is a rectangle with absolute ($) flags per edge. Coordinates are
// absolute positions; the flags only matter when a formula is copied.
// Column areas span all rows and row areas span all columns.
type Area struct {
	Kind                       AreaKind
	R0, C0, R1, C1             int
	AbsR0, AbsC0, AbsR1, AbsC1 bool
}

func CellAt(row, col int, absRow, absCol bool) Area {
	return Area{Kind: CellArea, R0: row, C0: col, R1: row, C1: col, AbsR0: absRow, AbsC0: absCol, AbsR1: absRow, AbsC1: absCol}
}

func (a Area) IsSingle() bool {
	return a.Kind == CellArea && a.R0 == a.R1 && a.C0 == a.C1
}

func (a Area) Range() model.Range {
	return model.Range{R0: a.R0, C0: a.C0, R1: a.R1, C1: a.C1}
}

// Span joins two areas into their bounding area (A1:B2 from A1 and B2),
// keeping the $ flags of the edges that end up on each side.
func Span(a, b Area) Area {
	out := Area{Kind: a.Kind}
	if a.Kind != b.Kind {
		out.Kind = CellArea
	}
	out.R0, out.AbsR0 = a.R0, a.AbsR0
	if b.R0 < a.R0 {
		out.R0, out.AbsR0 = b.R0, b.AbsR0
	}
	out.C0, out.AbsC0 = a.C0, a.AbsC0
	if b.C0 < a.C0 {
		out.C0, out.AbsC0 = b.C0, b.AbsC0
	}
	out.R1, out.AbsR1 = b.R1, b.AbsR1
	if a.R1 > b.R1 {
		out.R1, out.AbsR1 = a.R1, a.AbsR1
	}
	out.C1, out.AbsC1 = b.C1, b.AbsC1
	if a.C1 > b.C1 {
		out.C1, out.AbsC1 = a.C1, a.AbsC1
	}
	return out
}

// Walk calls fn for n and every descendant, depth first.
func Walk(n Node, fn func(Node)) {
	fn(n)
	switch x := n.(type) {
	case Paren:
		Walk(x.X, fn)
	case Unary:
		Walk(x.X, fn)
	case Percent:
		Walk(x.X, fn)
	case Binary:
		Walk(x.L, fn)
		Walk(x.R, fn)
	case Call:
		for _, a := range x.Args {
			Walk(a, fn)
		}
	case Array:
		for _, row := range x.Rows {
			for _, el := range row {
				Walk(el, fn)
			}
		}
	}
}

// ColName converts a 0-based column index to letters: 0 -> A, 26 -> AA.
func ColName(col int) string {
	var buf [4]byte
	i := len(buf)
	for n := col + 1; n > 0; n = (n - 1) / 26 {
		i--
		buf[i] = byte('A' + (n-1)%26)
	}
	return string(buf[i:])
}

// ColIndex converts column letters (case-insensitive) to a 0-based
// index; ok is false for invalid or out-of-range names.
func ColIndex(letters string) (int, bool) {
	if len(letters) == 0 || len(letters) > 3 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(letters); i++ {
		c := letters[i]
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		if c < 'A' || c > 'Z' {
			return 0, false
		}
		n = n*26 + int(c-'A'+1)
	}
	if n > model.MaxCols {
		return 0, false
	}
	return n - 1, true
}
