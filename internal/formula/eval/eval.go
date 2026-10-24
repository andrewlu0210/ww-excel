package eval

import (
	"math"
	"strings"
	"time"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
)

// Reader gives the evaluator access to workbook cells.
type Reader interface {
	Cell(sheet, row, col int) model.Cell
	SheetIndex(name string) (int, bool)
	// Each visits the non-empty cells of rg in row-major order.
	Each(sheet int, rg model.Range, fn func(row, col int, c model.Cell))
	// Used returns the used row/column counts of a sheet.
	Used(sheet int) (rows, cols int)
	SheetCount() int
	// Name resolves a defined name visible from sheet to its formula.
	Name(name string, sheet int) (ast.Node, bool)
	// Table resolves a structured reference in a formula at (sheet, row,
	// col) to an area, or returns an error code.
	Table(ref ast.TableRef, sheet, row, col int) (Ref, string)
}

// Evaluator evaluates one formula at a fixed position.
type Evaluator struct {
	R               Reader
	Sheet, Row, Col int
	Now             time.Time
	Rand            func() float64

	// Depth of nested defined names, to stop names defined in terms of
	// each other.
	nameDepth int

	// >0 while evaluating arguments of array functions (SUMPRODUCT):
	// multi-cell references then become arrays instead of being
	// intersected with the formula's row or column.
	arrayMode int
}

// Eval returns the scalar result of a formula.
func (e *Evaluator) Eval(n ast.Node) Value {
	v := e.eval(n)
	switch v.Kind {
	case KRef:
		return e.deref(v.Ref)
	case KArray:
		// No dynamic arrays (spilling) yet: show the top-left element.
		return v.Arr.At(0, 0)
	}
	return v
}

func (e *Evaluator) eval(n ast.Node) Value {
	switch x := n.(type) {
	case ast.Number:
		return Number(x.V)
	case ast.String:
		return Text(x.V)
	case ast.Bool:
		return Bool(x.V)
	case ast.Error:
		return Err(x.Code)
	case ast.Missing:
		return Empty
	case ast.Paren:
		return e.eval(x.X)
	case ast.Name:
		node, ok := e.R.Name(x.Name, e.Sheet)
		if !ok || e.nameDepth > 16 {
			return Err(model.ErrName)
		}
		e.nameDepth++
		defer func() { e.nameDepth-- }()
		return e.eval(node)
	case ast.Ref:
		return e.ref(x)
	case ast.TableRef:
		r, code := e.R.Table(x, e.Sheet, e.Row, e.Col)
		if code != "" {
			return Err(code)
		}
		return RefOf(&r)
	case ast.Array:
		arr := &Array{Rows: len(x.Rows), Cols: len(x.Rows[0])}
		for _, row := range x.Rows {
			for _, el := range row {
				arr.V = append(arr.V, e.eval(el))
			}
		}
		return ArrayOf(arr)
	case ast.Unary:
		v := e.operand(x.X)
		if x.Op == '+' {
			return v
		}
		return lift1(v, func(s Value) Value {
			n, err := ToNumber(s)
			if err.IsErr() {
				return err
			}
			return Number(-n)
		})
	case ast.Percent:
		return lift1(e.operand(x.X), func(s Value) Value {
			n, err := ToNumber(s)
			if err.IsErr() {
				return err
			}
			return Number(n / 100)
		})
	case ast.Binary:
		l := e.operand(x.L)
		r := e.operand(x.R)
		return lift2(l, r, func(a, b Value) Value { return binary(x.Op, a, b) })
	case ast.Call:
		def := Lookup(x.Name)
		if def == nil {
			return Err(model.ErrName)
		}
		if def.ArrayArgs {
			e.arrayMode++
			defer func() { e.arrayMode-- }()
		}
		return def.Fn(&Args{ev: e, nodes: x.Args, vals: make([]Value, len(x.Args)), done: make([]bool, len(x.Args))})
	}
	return Err(model.ErrValue)
}

// Reference evaluates a reference expression (INDIRECT) without
// dereferencing it.
func (e *Evaluator) Reference(n ast.Node) Value {
	return e.eval(n)
}

func (e *Evaluator) ref(x ast.Ref) Value {
	sheet := e.Sheet
	if x.Sheet != "" {
		idx, ok := e.R.SheetIndex(x.Sheet)
		if !ok {
			return Err(model.ErrRef)
		}
		sheet = idx
	}
	a := x.Area
	return RefOf(&Ref{Sheet: sheet, R0: a.R0, C0: a.C0, R1: a.R1, C1: a.C1})
}

// operand evaluates n for use by an operator: references become their
// value (implicit intersection), or arrays in array mode.
func (e *Evaluator) operand(n ast.Node) Value {
	v := e.eval(n)
	if v.Kind != KRef {
		return v
	}
	if e.arrayMode > 0 && !v.Ref.Single() {
		return e.toArray(v.Ref)
	}
	return e.deref(v.Ref)
}

// deref reads a reference as a scalar. A single cell is read directly;
// a range is intersected with the formula's row or column (legacy
// implicit intersection), else #VALUE!.
func (e *Evaluator) deref(r *Ref) Value {
	if r.Single() {
		return FromCell(e.R.Cell(r.Sheet, r.R0, r.C0))
	}
	if r.Sheet == e.Sheet {
		if r.C0 == r.C1 && e.Row >= r.R0 && e.Row <= r.R1 {
			return FromCell(e.R.Cell(r.Sheet, e.Row, r.C0))
		}
		if r.R0 == r.R1 && e.Col >= r.C0 && e.Col <= r.C1 {
			return FromCell(e.R.Cell(r.Sheet, r.R0, e.Col))
		}
	}
	return Err(model.ErrValue)
}

// clampUsed limits whole-column/row style areas to the sheet's used
// range so arrays and scans stay small. Cells outside are empty.
func (e *Evaluator) clampUsed(r *Ref) *Ref {
	rows, cols := e.R.Used(r.Sheet)
	out := *r
	if out.R1 >= rows && out.R1-out.R0 > 1000 {
		out.R1 = max(out.R0, rows-1)
	}
	if out.C1 >= cols && out.C1-out.C0 > 1000 {
		out.C1 = max(out.C0, cols-1)
	}
	return &out
}

func (e *Evaluator) toArray(r *Ref) Value {
	c := e.clampUsed(r)
	arr := &Array{Rows: c.Rows(), Cols: c.Cols(), V: make([]Value, c.Rows()*c.Cols())}
	e.R.Each(c.Sheet, c.Range(), func(row, col int, cell model.Cell) {
		arr.V[(row-c.R0)*arr.Cols+(col-c.C0)] = FromCell(cell)
	})
	return ArrayOf(arr)
}

func lift1(v Value, f func(Value) Value) Value {
	if v.Kind != KArray {
		return f(v)
	}
	out := &Array{Rows: v.Arr.Rows, Cols: v.Arr.Cols, V: make([]Value, len(v.Arr.V))}
	for i, el := range v.Arr.V {
		out.V[i] = f(el)
	}
	return ArrayOf(out)
}

// lift2 applies f element-wise, broadcasting scalars and single rows or
// columns; positions outside a smaller array are #N/A, as in Excel.
func lift2(a, b Value, f func(Value, Value) Value) Value {
	if a.Kind != KArray && b.Kind != KArray {
		return f(a, b)
	}
	ra, ca := dims(a)
	rb, cb := dims(b)
	rows, cols := max(ra, rb), max(ca, cb)
	out := &Array{Rows: rows, Cols: cols, V: make([]Value, rows*cols)}
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			out.V[r*cols+c] = f(elem(a, r, c), elem(b, r, c))
		}
	}
	return ArrayOf(out)
}

func dims(v Value) (int, int) {
	if v.Kind == KArray {
		return v.Arr.Rows, v.Arr.Cols
	}
	return 1, 1
}

func elem(v Value, r, c int) Value {
	if v.Kind != KArray {
		return v
	}
	a := v.Arr
	if a.Rows == 1 {
		r = 0
	}
	if a.Cols == 1 {
		c = 0
	}
	if r >= a.Rows || c >= a.Cols {
		return Err(model.ErrNA)
	}
	return a.At(r, c)
}

func binary(op string, a, b Value) Value {
	switch op {
	case "&":
		x, err := ToText(a)
		if err.IsErr() {
			return err
		}
		y, err := ToText(b)
		if err.IsErr() {
			return err
		}
		return Text(x + y)
	case "=", "<>", "<", ">", "<=", ">=":
		if a.IsErr() {
			return a
		}
		if b.IsErr() {
			return b
		}
		c := Compare(a, b)
		switch op {
		case "=":
			return Bool(c == 0)
		case "<>":
			return Bool(c != 0)
		case "<":
			return Bool(c < 0)
		case ">":
			return Bool(c > 0)
		case "<=":
			return Bool(c <= 0)
		default:
			return Bool(c >= 0)
		}
	}

	x, err := ToNumber(a)
	if err.IsErr() {
		return err
	}
	y, err := ToNumber(b)
	if err.IsErr() {
		return err
	}
	switch op {
	case "+":
		return Number(x + y)
	case "-":
		return Number(x - y)
	case "*":
		return Number(x * y)
	case "/":
		if y == 0 {
			return Err(model.ErrDiv0)
		}
		return Number(x / y)
	case "^":
		if x == 0 && y == 0 {
			return Err(model.ErrNum)
		}
		if x == 0 && y < 0 {
			return Err(model.ErrDiv0)
		}
		return Number(math.Pow(x, y))
	}
	return Err(model.ErrValue)
}

// IsVolatile reports whether n calls a volatile function (NOW, RAND...).
func IsVolatile(n ast.Node) bool {
	volatile := false
	ast.Walk(n, func(n ast.Node) {
		if c, ok := n.(ast.Call); ok {
			if d := Lookup(c.Name); d != nil && d.Volatile {
				volatile = true
			}
		}
	})
	return volatile
}

// lookupName normalizes function names, accepting the _xlfn. prefix
// that xlsx files use for newer functions.
func lookupName(name string) string {
	return strings.TrimPrefix(strings.ToUpper(name), "_XLFN.")
}
