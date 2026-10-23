package depgraph

import (
	"sort"
	"testing"

	"ww-excel/internal/model"
)

func deps(g *Graph, c Key) []Key {
	seen := map[Key]bool{}
	var out []Key
	g.Dependents(c, func(k Key) {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Row < out[j].Row })
	return out
}

func TestDependents(t *testing.T) {
	g := New()
	f1 := Key{0, 10, 0}
	f2 := Key{0, 11, 0}
	f3 := Key{0, 12, 0}
	g.Set(f1, []Area{{0, 0, 0, 0, 0}}, false)                // =A1
	g.Set(f2, []Area{{0, 0, 0, 9, 1}}, false)                // =SUM(A1:B10)
	g.Set(f3, []Area{{0, 0, 0, model.MaxRows - 1, 0}}, true) // =SUM(A:A), volatile
	g.Set(Key{1, 0, 0}, []Area{{1, 0, 0, 0, 0}}, false)      // other sheet

	if got := deps(g, Key{0, 0, 0}); len(got) != 3 {
		t.Fatalf("A1 dependents = %v, want 3", got)
	}
	if got := deps(g, Key{0, 5, 1}); len(got) != 1 || got[0] != f2 {
		t.Fatalf("B6 dependents = %v", got)
	}
	if got := deps(g, Key{0, 900_000, 0}); len(got) != 1 || got[0] != f3 {
		t.Fatalf("A900001 dependents = %v", got)
	}
	if got := deps(g, Key{0, 5, 2}); len(got) != 0 {
		t.Fatalf("C6 dependents = %v", got)
	}

	var vol []Key
	g.Volatile(func(k Key) { vol = append(vol, k) })
	if len(vol) != 1 || vol[0] != f3 {
		t.Fatalf("volatile = %v", vol)
	}

	// Re-registering replaces; removing cleans every index.
	g.Set(f2, []Area{{0, 50, 5, 50, 5}}, false)
	if got := deps(g, Key{0, 5, 1}); len(got) != 0 {
		t.Fatalf("stale range dependency: %v", got)
	}
	g.Remove(f1)
	g.Remove(f2)
	g.Remove(f3)
	g.Remove(Key{1, 0, 0})
	if g.Len() != 0 || len(g.singles) != 0 || len(g.buckets) != 0 || len(g.large) != 0 || len(g.ranges) != 0 {
		t.Fatalf("graph not empty: %+v", g)
	}
}
