// Package formula ties the parser, evaluator and dependency graph into
// a recalculation engine for a workbook.
//
// Recalculation after an edit:
//  1. Starting from the changed cells, walk dependents (BFS) and mark
//     every affected formula dirty, recording edges between them.
//  2. Evaluate the dirty formulas in topological order (Kahn), so each
//     formula sees up-to-date precedents. No recursion: chains of any
//     length are fine.
//  3. Whatever cannot be ordered is either part of a cycle or depends
//     on one. Strongly connected components identify the cycles; as in
//     Excel (without iterative calculation) circular cells show 0, and
//     the rest is evaluated afterwards.
package formula

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"time"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/depgraph"
	"ww-excel/internal/formula/eval"
	_ "ww-excel/internal/formula/functions" // register worksheet functions
	"ww-excel/internal/formula/parser"
	"ww-excel/internal/model"
)

type Key = depgraph.Key

type Engine struct {
	wb    *model.Workbook
	graph *depgraph.Graph
	// Now and Rand are injectable for tests.
	Now  func() time.Time
	Rand func() float64

	// Formula cells currently in a circular reference. Kept across
	// recalculations: a cycle stays reported until one of its cells is
	// recalculated without it.
	circular map[Key]bool

	// Parsed formulas of defined names, by formula text.
	names map[string]ast.Node
}

// compiled is stored in model.Formula.Compiled.
type compiled struct {
	node     ast.Node
	areas    []depgraph.Area
	volatile bool
	// unparsed: stored text this engine cannot parse (e.g. a structured
	// reference from an xlsx file). The cached value is kept as is.
	unparsed bool
}

func New(wb *model.Workbook) *Engine {
	return &Engine{wb: wb, graph: depgraph.New(), Now: time.Now, Rand: rand.Float64, circular: map[Key]bool{}, names: map[string]ast.Node{}}
}

// CompileError is a formula the user cannot enter, with a message for
// the user.
type CompileError struct{ Msg string }

func (e *CompileError) Error() string { return e.Msg }

// Compile parses formula text (without "=") typed into a cell on sheet
// and returns the canonical formula. Unknown functions are allowed and
// evaluate to #NAME?, as in Excel; wrong argument counts are rejected.
func (e *Engine) Compile(text string, sheet int) (*model.Formula, error) {
	node, err := parser.Parse(text)
	if err != nil {
		return nil, &CompileError{Msg: err.Error()}
	}
	if err := checkCalls(node); err != nil {
		return nil, err
	}
	return &model.Formula{Text: ast.Format(node), Compiled: e.compile(node, sheet)}, nil
}

// CompileNode builds a formula from an AST, e.g. a translated copy.
func (e *Engine) CompileNode(node ast.Node, sheet int) *model.Formula {
	return &model.Formula{Text: ast.Format(node), Compiled: e.compile(node, sheet)}
}

// Parsed returns the AST of a formula, parsing it if needed.
func (e *Engine) Parsed(f *model.Formula, sheet int) ast.Node {
	return e.ensure(f, sheet).node
}

func checkCalls(node ast.Node) error {
	var err error
	ast.Walk(node, func(n ast.Node) {
		c, ok := n.(ast.Call)
		if !ok || err != nil {
			return
		}
		d := eval.Lookup(c.Name)
		if d == nil {
			return
		}
		switch {
		case len(c.Args) < d.Min:
			err = &CompileError{Msg: fmt.Sprintf("函數 %s 的引數太少（至少需要 %d 個）", c.Name, d.Min)}
		case d.Max >= 0 && len(c.Args) > d.Max:
			err = &CompileError{Msg: fmt.Sprintf("函數 %s 的引數太多（最多 %d 個）", c.Name, d.Max)}
		}
	})
	return err
}

func (e *Engine) compile(node ast.Node, sheet int) *compiled {
	c := &compiled{node: node, volatile: eval.IsVolatile(node)}
	e.collectAreas(node, sheet, c, 0)
	return c
}

// collectAreas records the areas a formula depends on, following
// defined names to the references they stand for.
func (e *Engine) collectAreas(node ast.Node, sheet int, c *compiled, depth int) {
	ast.Walk(node, func(n ast.Node) {
		switch r := n.(type) {
		case ast.Ref:
			s := sheet
			if r.Sheet != "" {
				idx, ok := e.sheetIndex(r.Sheet)
				if !ok {
					return // evaluates to #REF!
				}
				s = idx
			}
			a := r.Area
			c.areas = append(c.areas, depgraph.Area{Sheet: s, R0: a.R0, C0: a.C0, R1: a.R1, C1: a.C1})
		case ast.TableRef:
			if r.Table == "" {
				// [@Col] inside a table: the position is not known here,
				// so depend on that column of every table on the sheet
				// (not the whole table, which would include the formula).
				if sheet >= 0 && sheet < len(e.wb.Sheets) {
					for _, t := range e.wb.Sheets[sheet].Layout.Tables {
						if ref, code := e.table(ast.TableRef{Table: t.Name, Spec: r.Spec}, sheet, -1, -1); code == "" {
							c.areas = append(c.areas, depgraph.Area{Sheet: ref.Sheet, R0: ref.R0, C0: ref.C0, R1: ref.R1, C1: ref.C1})
						}
					}
				}
				return
			}
			if ref, code := e.table(r, sheet, -1, -1); code == "" {
				c.areas = append(c.areas, depgraph.Area{Sheet: ref.Sheet, R0: ref.R0, C0: ref.C0, R1: ref.R1, C1: ref.C1})
			}
		case ast.Name:
			if depth > 16 {
				return
			}
			if target, ok := e.name(r.Name, sheet); ok {
				c.volatile = c.volatile || eval.IsVolatile(target)
				e.collectAreas(target, sheet, c, depth+1)
			}
		}
	})
}

// Table resolves a structured reference in a formula at a position.
func (e *Engine) Table(ref ast.TableRef, sheet, row, col int) (eval.Ref, string) {
	return e.table(ref, sheet, row, col)
}

// table resolves a structured reference. row < 0 means the formula's
// position is unknown (dependency tracking): "this row" then covers
// all data rows.
func (e *Engine) table(ref ast.TableRef, sheet, row, col int) (eval.Ref, string) {
	var t *model.Table
	var sh *model.Sheet
	if ref.Table == "" {
		if sheet < 0 || sheet >= len(e.wb.Sheets) {
			return eval.Ref{}, model.ErrRef
		}
		sh = e.wb.Sheets[sheet]
		t = sh.Layout.TableAt(row, col)
	} else {
		t, sh = e.wb.FindTable(ref.Table)
	}
	if t == nil {
		return eval.Ref{}, model.ErrRef
	}
	spec, err := ast.ParseTableSpec(ref.Spec)
	if err != nil {
		return eval.Ref{}, model.ErrRef
	}
	si, _ := e.sheetIndex(sh.Name)
	out := eval.Ref{Sheet: si, C0: t.Range.C0, C1: t.Range.C1}
	if spec.Col0 != "" {
		c0, ok := t.ColumnIndex(sh, spec.Col0)
		if !ok {
			return eval.Ref{}, model.ErrRef
		}
		c1 := c0
		if spec.Col1 != "" {
			if c1, ok = t.ColumnIndex(sh, spec.Col1); !ok {
				return eval.Ref{}, model.ErrRef
			}
		}
		out.C0, out.C1 = min(c0, c1), max(c0, c1)
	}
	data := t.DataRows()
	switch {
	case spec.ThisRow:
		if row < 0 {
			out.R0, out.R1 = data.R0, max(data.R0, data.R1)
		} else if row < data.R0 || row > data.R1 {
			return eval.Ref{}, model.ErrValue
		} else {
			out.R0, out.R1 = row, row
		}
	default:
		headers, rows, totals := spec.Headers, spec.Data, spec.Totals
		if !headers && !rows && !totals {
			rows = true
		}
		if totals && !t.TotalsRow && !rows && !headers {
			return eval.Ref{}, model.ErrRef
		}
		out.R0, out.R1 = data.R0, data.R1
		if headers {
			out.R0 = t.Range.R0
			if !rows {
				out.R1 = t.Range.R0
			}
		}
		if totals && t.TotalsRow {
			out.R1 = t.Range.R1
			if !rows && !headers {
				out.R0 = t.Range.R1
			}
		}
		if out.R1 < out.R0 {
			return eval.Ref{}, model.ErrRef
		}
	}
	return out, ""
}

// name parses the formula of a defined name visible from sheet.
func (e *Engine) name(name string, sheet int) (ast.Node, bool) {
	var sh *model.Sheet
	if sheet >= 0 && sheet < len(e.wb.Sheets) {
		sh = e.wb.Sheets[sheet]
	}
	d := e.wb.LookupName(name, sh)
	if d == nil {
		return nil, false
	}
	if n, ok := e.names[d.Formula]; ok {
		return n, true
	}
	n, err := parser.Parse(d.Formula)
	if err != nil {
		n = ast.Error{Code: model.ErrName}
	}
	e.names[d.Formula] = n
	return n, true
}

// ensure returns the compiled form of f, compiling stored text (e.g.
// from a loaded file). Text that cannot be parsed keeps its cached value.
func (e *Engine) ensure(f *model.Formula, sheet int) *compiled {
	if c, ok := f.Compiled.(*compiled); ok {
		return c
	}
	node, err := parser.Parse(f.Text)
	if err != nil {
		c := &compiled{node: ast.Error{Code: model.ErrName}, unparsed: true}
		f.Compiled = c
		return c
	}
	c := e.compile(node, sheet)
	f.Compiled = c
	return c
}

func (e *Engine) sheetIndex(name string) (int, bool) {
	for i, s := range e.wb.Sheets {
		if strings.EqualFold(s.Name, name) {
			return i, true
		}
	}
	return 0, false
}

// Rebuild registers every formula in the workbook and recalculates all
// of them. Returns the formula cells whose values changed.
func (e *Engine) Rebuild() []Key {
	e.graph = depgraph.New()
	e.circular = map[Key]bool{}
	var all []Key
	for si, sh := range e.wb.Sheets {
		sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			if c.F == nil {
				return
			}
			// Sheets may have been added, removed or moved: resolve sheet
			// references again.
			if old, ok := c.F.Compiled.(*compiled); ok && !old.unparsed {
				c.F.Compiled = e.compile(old.node, si)
			}
			all = append(all, Key{Sheet: si, Row: row, Col: col})
		})
	}
	return e.Update(all)
}

// Update is called after the given cells changed (values or formulas).
// It re-registers formulas among them and recalculates everything that
// depends on them, plus volatile formulas. Returns the formula cells
// whose values changed.
func (e *Engine) Update(changed []Key) []Key {
	for _, k := range changed {
		c := e.cell(k)
		if c.F != nil {
			comp := e.ensure(c.F, k.Sheet)
			e.graph.Set(k, comp.areas, comp.volatile)
		} else if e.graph.Has(k) {
			e.graph.Remove(k)
			delete(e.circular, k)
		}
	}
	return e.recalc(changed)
}

// Circular returns the formula cells in circular references, sorted.
func (e *Engine) Circular() []Key {
	out := make([]Key, 0, len(e.circular))
	for k := range e.circular {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return lessKey(out[i], out[j]) })
	return out
}

// Formulas returns the number of registered formulas.
func (e *Engine) Formulas() int { return e.graph.Len() }

func (e *Engine) cell(k Key) model.Cell {
	if k.Sheet < 0 || k.Sheet >= len(e.wb.Sheets) {
		return model.Cell{}
	}
	return e.wb.Sheets[k.Sheet].Get(k.Row, k.Col)
}

func (e *Engine) recalc(seeds []Key) []Key {
	dirty := map[Key]bool{}
	var queue []Key
	mark := func(k Key) {
		if !dirty[k] {
			dirty[k] = true
			queue = append(queue, k)
		}
	}
	// Changed formulas are dirty themselves; changed values only seed.
	seen := map[Key]bool{}
	for _, k := range seeds {
		if seen[k] {
			continue
		}
		seen[k] = true
		if e.graph.Has(k) {
			mark(k)
		} else {
			queue = append(queue, k)
		}
	}
	e.graph.Volatile(mark)

	// BFS over dependents, recording edges between dirty formulas.
	edges := map[Key][]Key{}
	indeg := map[Key]int{}
	for i := 0; i < len(queue); i++ {
		x := queue[i]
		local := map[Key]bool{}
		e.graph.Dependents(x, func(d Key) {
			if local[d] {
				return
			}
			local[d] = true
			if dirty[x] {
				edges[x] = append(edges[x], d)
				indeg[d]++
			}
			mark(d)
		})
	}

	// Dirty formulas in discovery order, for deterministic evaluation.
	var order []Key
	for _, k := range queue {
		if dirty[k] {
			order = append(order, k)
		}
	}

	var changed []Key
	done := map[Key]bool{}
	evaluate := func(k Key) {
		done[k] = true
		delete(e.circular, k)
		if e.evalCell(k) {
			changed = append(changed, k)
		}
	}
	kahn := func(nodes []Key, indeg map[Key]int) {
		var ready []Key
		for _, k := range nodes {
			if indeg[k] == 0 {
				ready = append(ready, k)
			}
		}
		for len(ready) > 0 {
			k := ready[0]
			ready = ready[1:]
			evaluate(k)
			for _, d := range edges[k] {
				if done[d] {
					continue
				}
				if _, tracked := indeg[d]; !tracked {
					continue
				}
				indeg[d]--
				if indeg[d] == 0 {
					ready = append(ready, d)
				}
			}
		}
	}
	kahn(order, indeg)

	var rest []Key
	for _, k := range order {
		if !done[k] {
			rest = append(rest, k)
		}
	}
	if len(rest) == 0 {
		return changed
	}

	// Cycles: cells in a strongly connected component (or with a self
	// reference) show 0; the cells downstream are evaluated after.
	inCycle := cycleMembers(rest, edges, done)
	for _, k := range rest {
		if inCycle[k] {
			done[k] = true
			e.circular[k] = true
			if e.setValue(k, model.NumberCell(0)) {
				changed = append(changed, k)
			}
		}
	}

	var downstream []Key
	indeg2 := map[Key]int{}
	for _, k := range rest {
		if !inCycle[k] {
			downstream = append(downstream, k)
			indeg2[k] = 0
		}
	}
	for _, k := range downstream {
		for _, d := range edges[k] {
			if _, ok := indeg2[d]; ok {
				indeg2[d]++
			}
		}
	}
	kahn(downstream, indeg2)
	return changed
}

func lessKey(a, b Key) bool {
	if a.Sheet != b.Sheet {
		return a.Sheet < b.Sheet
	}
	if a.Row != b.Row {
		return a.Row < b.Row
	}
	return a.Col < b.Col
}

// cycleMembers finds nodes on cycles among nodes (iterative Tarjan).
func cycleMembers(nodes []Key, edges map[Key][]Key, done map[Key]bool) map[Key]bool {
	in := map[Key]bool{}
	for _, k := range nodes {
		in[k] = true
	}
	index := map[Key]int{}
	low := map[Key]int{}
	onStack := map[Key]bool{}
	var stack []Key
	members := map[Key]bool{}
	next := 0

	type frame struct {
		k Key
		i int
	}
	for _, root := range nodes {
		if _, seen := index[root]; seen {
			continue
		}
		call := []frame{{root, 0}}
		index[root], low[root] = next, next
		next++
		stack = append(stack, root)
		onStack[root] = true
		for len(call) > 0 {
			f := &call[len(call)-1]
			out := edges[f.k]
			if f.i < len(out) {
				w := out[f.i]
				f.i++
				if !in[w] || done[w] {
					continue
				}
				if _, seen := index[w]; !seen {
					index[w], low[w] = next, next
					next++
					stack = append(stack, w)
					onStack[w] = true
					call = append(call, frame{w, 0})
				} else if onStack[w] {
					low[f.k] = min(low[f.k], index[w])
				}
				continue
			}
			// All edges of f.k visited.
			if low[f.k] == index[f.k] {
				var comp []Key
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					comp = append(comp, w)
					if w == f.k {
						break
					}
				}
				if len(comp) > 1 || selfLoop(f.k, edges) {
					for _, w := range comp {
						members[w] = true
					}
				}
			}
			k := f.k
			call = call[:len(call)-1]
			if len(call) > 0 {
				parent := call[len(call)-1].k
				low[parent] = min(low[parent], low[k])
			}
		}
	}
	return members
}

func selfLoop(k Key, edges map[Key][]Key) bool {
	for _, d := range edges[k] {
		if d == k {
			return true
		}
	}
	return false
}

// evalCell recalculates one formula cell; reports whether its value
// changed.
func (e *Engine) evalCell(k Key) bool {
	c := e.cell(k)
	if c.F == nil {
		return false
	}
	comp := e.ensure(c.F, k.Sheet)
	if comp.unparsed {
		return false
	}
	ev := eval.Evaluator{R: reader{e}, Sheet: k.Sheet, Row: k.Row, Col: k.Col, Now: e.Now(), Rand: e.Rand}
	return e.setValue(k, ev.Eval(comp.node).ToCell())
}

func (e *Engine) setValue(k Key, v model.Cell) bool {
	sh := e.wb.Sheets[k.Sheet]
	old := sh.Get(k.Row, k.Col)
	v.F, v.S = old.F, old.S // keep the formula and the cell's format
	if v.SameValue(old) {
		return false
	}
	sh.Set(k.Row, k.Col, v)
	return true
}

// EvalAt evaluates a formula as if it were in (sheet, row, col), e.g.
// a validation or conditional format rule. References are read as
// values.
func (e *Engine) EvalAt(node ast.Node, sheet, row, col int) eval.Value {
	ev := eval.Evaluator{R: reader{e}, Sheet: sheet, Row: row, Col: col, Now: e.Now(), Rand: e.Rand}
	return ev.Eval(node)
}

// ReferenceAt evaluates a formula without reading references, so a
// list source such as $H$1:$H$5 stays a range.
func (e *Engine) ReferenceAt(node ast.Node, sheet, row, col int) eval.Value {
	ev := eval.Evaluator{R: reader{e}, Sheet: sheet, Row: row, Col: col, Now: e.Now(), Rand: e.Rand}
	return ev.Reference(node)
}

// reader adapts the workbook to eval.Reader.
type reader struct{ e *Engine }

func (r reader) Cell(sheet, row, col int) model.Cell {
	if row < 0 || row >= model.MaxRows || col < 0 || col >= model.MaxCols {
		return model.Cell{}
	}
	return r.e.cell(Key{Sheet: sheet, Row: row, Col: col})
}

func (r reader) SheetIndex(name string) (int, bool) { return r.e.sheetIndex(name) }
func (r reader) SheetCount() int                    { return len(r.e.wb.Sheets) }
func (r reader) Table(ref ast.TableRef, sheet, row, col int) (eval.Ref, string) {
	return r.e.table(ref, sheet, row, col)
}

func (r reader) Name(name string, sheet int) (ast.Node, bool) {
	return r.e.name(name, sheet)
}

func (r reader) Each(sheet int, rg model.Range, fn func(row, col int, c model.Cell)) {
	if sheet < 0 || sheet >= len(r.e.wb.Sheets) {
		return
	}
	r.e.wb.Sheets[sheet].EachInRange(rg, fn)
}

func (r reader) Used(sheet int) (int, int) {
	if sheet < 0 || sheet >= len(r.e.wb.Sheets) {
		return 0, 0
	}
	return r.e.wb.Sheets[sheet].UsedRange()
}

// FuncInfo describes a function for autocomplete and parameter hints.
type FuncInfo struct {
	Name     string `json:"name"`
	Sig      string `json:"sig"`
	Desc     string `json:"desc"`
	Category string `json:"category"`
}

func Functions() []FuncInfo {
	defs := eval.Functions()
	out := make([]FuncInfo, len(defs))
	for i, d := range defs {
		out[i] = FuncInfo{Name: d.Name, Sig: d.Sig, Desc: d.Desc, Category: d.Category}
	}
	return out
}
