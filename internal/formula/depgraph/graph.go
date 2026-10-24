// Package depgraph indexes which formulas depend on which cells.
//
// Single-cell precedents live in a map. Range precedents are bucketed
// by the storage blocks they cover, so a changed cell only checks the
// ranges overlapping its block; ranges covering many blocks (A:A) are
// kept in a short list that is scanned.
package depgraph

import "ww-excel/internal/model"

type Key struct{ Sheet, Row, Col int }

// ck is the compact form of Key used inside the graph: millions of
// entries are stored, so 12 bytes instead of 24 matter.
type ck struct{ s, r, c int32 }

func compact(k Key) ck { return ck{int32(k.Sheet), int32(k.Row), int32(k.Col)} }
func (k ck) key() Key  { return Key{int(k.s), int(k.r), int(k.c)} }

type Area struct {
	Sheet          int
	R0, C0, R1, C1 int
}

func (a Area) contains(k Key) bool {
	return k.Sheet == a.Sheet && k.Row >= a.R0 && k.Row <= a.R1 && k.Col >= a.C0 && k.Col <= a.C1
}

func (a Area) single() bool { return a.R0 == a.R1 && a.C0 == a.C1 }

// Ranges spanning more blocks than this go to the scanned list.
const maxBuckets = 64

type bucket struct {
	sheet  int
	br, bc int
}

type rangeDep struct {
	area    Area
	dep     Key
	buckets []bucket // nil when in the large list
}

type registration struct {
	singles []ck
	ranges  []int
}

type Graph struct {
	// Formulas referencing each single cell. Most cells have one or
	// two dependents, so a slice is far smaller than a set.
	singles  map[ck][]ck
	ranges   map[int]*rangeDep
	buckets  map[bucket]map[int]struct{}
	large    map[int]struct{}
	regs     map[ck]registration
	volatile map[ck]struct{}
	nextID   int
}

func New() *Graph {
	return &Graph{
		singles:  map[ck][]ck{},
		ranges:   map[int]*rangeDep{},
		buckets:  map[bucket]map[int]struct{}{},
		large:    map[int]struct{}{},
		regs:     map[ck]registration{},
		volatile: map[ck]struct{}{},
	}
}

// Set registers formula f with its precedent areas, replacing any
// previous registration.
func (g *Graph) Set(fk Key, areas []Area, volatile bool) {
	g.Remove(fk)
	f := compact(fk)
	var reg registration
	for _, a := range areas {
		if a.single() {
			k := ck{int32(a.Sheet), int32(a.R0), int32(a.C0)}
			deps := g.singles[k]
			if !containsCK(deps, f) {
				g.singles[k] = append(deps, f)
			}
			reg.singles = append(reg.singles, k)
			continue
		}
		id := g.nextID
		g.nextID++
		rd := &rangeDep{area: a, dep: fk}
		br0, br1 := a.R0/model.BlockRows, a.R1/model.BlockRows
		bc0, bc1 := a.C0/model.BlockCols, a.C1/model.BlockCols
		if (br1-br0+1)*(bc1-bc0+1) > maxBuckets {
			g.large[id] = struct{}{}
		} else {
			for br := br0; br <= br1; br++ {
				for bc := bc0; bc <= bc1; bc++ {
					b := bucket{a.Sheet, br, bc}
					set := g.buckets[b]
					if set == nil {
						set = map[int]struct{}{}
						g.buckets[b] = set
					}
					set[id] = struct{}{}
					rd.buckets = append(rd.buckets, b)
				}
			}
		}
		g.ranges[id] = rd
		reg.ranges = append(reg.ranges, id)
	}
	g.regs[f] = reg
	if volatile {
		g.volatile[f] = struct{}{}
	}
}

// Remove unregisters formula f.
func (g *Graph) Remove(fk Key) {
	f := compact(fk)
	reg, ok := g.regs[f]
	if !ok {
		return
	}
	for _, k := range reg.singles {
		deps := g.singles[k]
		for i, d := range deps {
			if d == f {
				deps = append(deps[:i:i], deps[i+1:]...)
				break
			}
		}
		if len(deps) == 0 {
			delete(g.singles, k)
		} else {
			g.singles[k] = deps
		}
	}
	for _, id := range reg.ranges {
		rd := g.ranges[id]
		for _, b := range rd.buckets {
			if set := g.buckets[b]; set != nil {
				delete(set, id)
				if len(set) == 0 {
					delete(g.buckets, b)
				}
			}
		}
		delete(g.large, id)
		delete(g.ranges, id)
	}
	delete(g.regs, f)
	delete(g.volatile, f)
}

func (g *Graph) Has(f Key) bool { _, ok := g.regs[compact(f)]; return ok }

func containsCK(list []ck, k ck) bool {
	for _, x := range list {
		if x == k {
			return true
		}
	}
	return false
}

// Len is the number of registered formulas.
func (g *Graph) Len() int { return len(g.regs) }

// Dependents calls fn for every formula that references cell c. A
// formula referencing c through several areas may be reported more
// than once.
func (g *Graph) Dependents(c Key, fn func(Key)) {
	for _, f := range g.singles[compact(c)] {
		fn(f.key())
	}
	b := bucket{c.Sheet, c.Row / model.BlockRows, c.Col / model.BlockCols}
	for id := range g.buckets[b] {
		if rd := g.ranges[id]; rd.area.contains(c) {
			fn(rd.dep)
		}
	}
	for id := range g.large {
		if rd := g.ranges[id]; rd.area.contains(c) {
			fn(rd.dep)
		}
	}
}

// Volatile calls fn for every volatile formula.
func (g *Graph) Volatile(fn func(Key)) {
	for f := range g.volatile {
		fn(f.key())
	}
}

// Each calls fn for every registered formula.
func (g *Graph) Each(fn func(Key)) {
	for f := range g.regs {
		fn(f.key())
	}
}
