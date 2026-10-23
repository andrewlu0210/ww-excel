package eval

import (
	"math"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
)

// Args gives a function lazy access to its arguments: each argument is
// evaluated on first use, so IF only evaluates the branch it returns.
type Args struct {
	ev    *Evaluator
	nodes []ast.Node
	vals  []Value
	done  []bool
}

func (a *Args) Len() int            { return len(a.nodes) }
func (a *Args) Ev() *Evaluator      { return a.ev }
func (a *Args) Node(i int) ast.Node { return a.nodes[i] }

// Has reports whether argument i was given and not left empty.
func (a *Args) Has(i int) bool {
	if i >= len(a.nodes) {
		return false
	}
	_, missing := a.nodes[i].(ast.Missing)
	return !missing
}

// Raw evaluates argument i; references stay references.
func (a *Args) Raw(i int) Value {
	if i >= len(a.nodes) {
		return Empty
	}
	if !a.done[i] {
		a.vals[i] = a.ev.eval(a.nodes[i])
		a.done[i] = true
	}
	return a.vals[i]
}

// Scalar evaluates argument i to a single value. In array mode arrays
// are returned as is.
func (a *Args) Scalar(i int) Value {
	v := a.Raw(i)
	switch v.Kind {
	case KRef:
		if a.ev.arrayMode > 0 && !v.Ref.Single() {
			return a.ev.toArray(v.Ref)
		}
		return a.ev.deref(v.Ref)
	case KArray:
		if a.ev.arrayMode > 0 {
			return v
		}
		return v.Arr.At(0, 0)
	}
	return v
}

// Number returns argument i as a number; the second result is an error
// value when coercion fails. A missing argument is def.
func (a *Args) Number(i int, def float64) (float64, Value) {
	if !a.Has(i) {
		return def, Empty
	}
	return ToNumber(a.Scalar(i))
}

// Int returns argument i truncated toward zero.
func (a *Args) Int(i int, def int) (int, Value) {
	n, err := a.Number(i, float64(def))
	if err.IsErr() {
		return 0, err
	}
	if math.Abs(n) > 1e15 {
		return 0, Err(model.ErrNum)
	}
	return int(math.Trunc(n)), Empty
}

func (a *Args) Text(i int, def string) (string, Value) {
	if !a.Has(i) {
		return def, Empty
	}
	return ToText(a.Scalar(i))
}

func (a *Args) Bool(i int, def bool) (bool, Value) {
	if !a.Has(i) {
		return def, Empty
	}
	return ToBool(a.Scalar(i))
}

// EachValue visits every value of every argument from index from:
// cells of references and elements of arrays (fromRef = true), or the
// scalar itself. Empty cells of references are not visited. Returning
// false stops.
func (a *Args) EachValue(from int, fn func(v Value, fromRef bool) bool) {
	for i := from; i < a.Len(); i++ {
		if !a.Has(i) {
			if !fn(Empty, false) {
				return
			}
			continue
		}
		if !a.eachIn(a.Raw(i), fn) {
			return
		}
	}
}

func (a *Args) eachIn(v Value, fn func(v Value, fromRef bool) bool) bool {
	switch v.Kind {
	case KRef:
		cont := true
		a.ev.R.Each(v.Ref.Sheet, v.Ref.Range(), func(_, _ int, c model.Cell) {
			if cont {
				cont = fn(FromCell(c), true)
			}
		})
		return cont
	case KArray:
		for _, el := range v.Arr.V {
			if !fn(el, true) {
				return false
			}
		}
		return true
	}
	return fn(v, false)
}

// Numbers visits numbers per Excel's aggregate rules (SUM, AVERAGE...):
// from references and arrays only numbers count (text, logicals and
// empties are skipped); direct arguments are coerced, so SUM("2", TRUE)
// is 3. The first error is returned.
func (a *Args) Numbers(from int, fn func(float64)) Value {
	var errv Value
	a.EachValue(from, func(v Value, fromRef bool) bool {
		switch {
		case v.Kind == KError:
			errv = v
			return false
		case v.Kind == KNumber:
			fn(v.Num)
		case fromRef:
			// skip text, logicals and empties inside ranges
		case v.Kind == KEmpty:
			// an omitted argument counts as nothing
		default:
			n, err := ToNumber(v)
			if err.IsErr() {
				errv = err
				return false
			}
			fn(n)
		}
		return true
	})
	return errv
}

// Grid is a 2-D view of a reference or array argument, used by lookup
// functions. Whole-column references are limited to the used range.
type Grid struct {
	ev         *Evaluator
	ref        *Ref
	arr        *Array
	scalar     Value
	rows, cols int
}

func (a *Args) Grid(i int) (Grid, Value) {
	v := a.Raw(i)
	switch v.Kind {
	case KRef:
		r := a.ev.clampUsed(v.Ref)
		return Grid{ev: a.ev, ref: r, rows: r.Rows(), cols: r.Cols()}, Empty
	case KArray:
		return Grid{arr: v.Arr, rows: v.Arr.Rows, cols: v.Arr.Cols}, Empty
	case KError:
		return Grid{}, v
	}
	return Grid{scalar: v, rows: 1, cols: 1}, Empty
}

func (g Grid) Rows() int { return g.rows }
func (g Grid) Cols() int { return g.cols }

func (g Grid) At(r, c int) Value {
	switch {
	case g.ref != nil:
		return FromCell(g.ev.R.Cell(g.ref.Sheet, g.ref.R0+r, g.ref.C0+c))
	case g.arr != nil:
		return g.arr.At(r, c)
	}
	return g.scalar
}

// IsRef reports whether the grid comes from a reference, and returns it.
func (g Grid) IsRef() (*Ref, bool) { return g.ref, g.ref != nil }
