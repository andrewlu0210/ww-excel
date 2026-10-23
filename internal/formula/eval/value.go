// Package eval evaluates formula ASTs against a workbook, following
// Excel's value types and coercion rules.
package eval

import (
	"math"
	"strings"

	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

type Kind uint8

const (
	KEmpty Kind = iota
	KNumber
	KString
	KBool
	KError
	KArray
	KRef
)

// Value is the result of evaluating a node. Ref values are produced by
// references and only dereferenced where a scalar is needed, so
// functions such as SUM can walk ranges.
type Value struct {
	Kind Kind
	Num  float64 // number, or 1/0 for bool
	Str  string  // text or error code
	Arr  *Array
	Ref  *Ref
}

type Array struct {
	Rows, Cols int
	V          []Value
}

func (a *Array) At(r, c int) Value { return a.V[r*a.Cols+c] }

// Ref is a resolved rectangular area on a sheet.
type Ref struct {
	Sheet          int
	R0, C0, R1, C1 int
}

func (r *Ref) Rows() int    { return r.R1 - r.R0 + 1 }
func (r *Ref) Cols() int    { return r.C1 - r.C0 + 1 }
func (r *Ref) Single() bool { return r.R0 == r.R1 && r.C0 == r.C1 }
func (r *Ref) Range() model.Range {
	return model.Range{R0: r.R0, C0: r.C0, R1: r.R1, C1: r.C1}
}

var Empty = Value{}

// Number returns a number value; NaN and infinities become #NUM!.
func Number(n float64) Value {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return Err(model.ErrNum)
	}
	return Value{Kind: KNumber, Num: n}
}

func Text(s string) Value     { return Value{Kind: KString, Str: s} }
func Err(code string) Value   { return Value{Kind: KError, Str: code} }
func ArrayOf(a *Array) Value  { return Value{Kind: KArray, Arr: a} }
func RefOf(r *Ref) Value      { return Value{Kind: KRef, Ref: r} }
func (v Value) IsErr() bool   { return v.Kind == KError }
func (v Value) IsEmpty() bool { return v.Kind == KEmpty }

func Bool(b bool) Value {
	if b {
		return Value{Kind: KBool, Num: 1}
	}
	return Value{Kind: KBool}
}

func FromCell(c model.Cell) Value {
	switch c.Kind {
	case model.KindNumber:
		return Value{Kind: KNumber, Num: c.Num}
	case model.KindString:
		return Text(c.Str)
	case model.KindBool:
		return Bool(c.Num != 0)
	case model.KindError:
		return Err(c.Str)
	}
	return Empty
}

// ToCell converts a scalar result into a cell value. An empty result
// (e.g. =A1 with A1 empty) shows as 0, as in Excel.
func (v Value) ToCell() model.Cell {
	switch v.Kind {
	case KNumber:
		return model.NumberCell(v.Num)
	case KString:
		return model.StringCell(v.Str)
	case KBool:
		return model.BoolCell(v.Num != 0)
	case KError:
		return model.ErrorCell(v.Str)
	}
	return model.NumberCell(0)
}

// ToNumber coerces a scalar to a number: TRUE is 1, empty is 0, numeric
// text is parsed. Errors are returned as the second value.
func ToNumber(v Value) (float64, Value) {
	switch v.Kind {
	case KNumber, KBool:
		return v.Num, Empty
	case KEmpty:
		return 0, Empty
	case KString:
		if n, ok := model.ParseNumber(v.Str); ok {
			return n, Empty
		}
		return 0, Err(model.ErrValue)
	case KError:
		return 0, v
	}
	return 0, Err(model.ErrValue)
}

// ToText coerces a scalar to text.
func ToText(v Value) (string, Value) {
	switch v.Kind {
	case KString:
		return v.Str, Empty
	case KNumber:
		return numfmt.General(v.Num), Empty
	case KBool:
		if v.Num != 0 {
			return "TRUE", Empty
		}
		return "FALSE", Empty
	case KEmpty:
		return "", Empty
	case KError:
		return "", v
	}
	return "", Err(model.ErrValue)
}

// ToBool coerces a scalar to a logical value.
func ToBool(v Value) (bool, Value) {
	switch v.Kind {
	case KBool, KNumber:
		return v.Num != 0, Empty
	case KEmpty:
		return false, Empty
	case KString:
		switch strings.ToUpper(v.Str) {
		case "TRUE":
			return true, Empty
		case "FALSE":
			return false, Empty
		}
		return false, Err(model.ErrValue)
	case KError:
		return false, v
	}
	return false, Err(model.ErrValue)
}

// typeRank orders types for comparison: numbers < text < logicals.
func typeRank(k Kind) int {
	switch k {
	case KString:
		return 1
	case KBool:
		return 2
	}
	return 0
}

// Compare orders two non-error scalars the way Excel's comparison
// operators do. Empty compares as 0, "" or FALSE depending on the other
// side; text compares case-insensitively; numbers compare at 15
// significant digits so 0.1+0.2=0.3 is TRUE.
func Compare(a, b Value) int {
	if a.Kind == KEmpty {
		a = emptyLike(b.Kind)
	}
	if b.Kind == KEmpty {
		b = emptyLike(a.Kind)
	}
	ra, rb := typeRank(a.Kind), typeRank(b.Kind)
	if ra != rb {
		return cmpInt(ra, rb)
	}
	switch a.Kind {
	case KString:
		return strings.Compare(strings.ToLower(a.Str), strings.ToLower(b.Str))
	default:
		x, y := numfmt.Normalize(a.Num), numfmt.Normalize(b.Num)
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
}

func emptyLike(k Kind) Value {
	switch k {
	case KString:
		return Text("")
	case KBool:
		return Bool(false)
	}
	return Value{Kind: KNumber}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
