package eval

import (
	"sort"
	"sync"
)

type Fn func(a *Args) Value

// Def describes a worksheet function.
type Def struct {
	Name string
	// Min and Max argument counts; Max < 0 means variadic.
	Min, Max int
	Fn       Fn
	// Volatile functions recalculate on every change (NOW, RAND).
	Volatile bool
	// ArrayArgs evaluates arguments in array mode (SUMPRODUCT).
	ArrayArgs bool
	// Sig is the signature shown in parameter hints, e.g.
	// "number1, [number2], ...". Desc is a one-line description.
	Sig, Desc string
	Category  string
}

var (
	mu       sync.RWMutex
	registry = map[string]*Def{}
)

// Register adds a function; called from init functions.
func Register(d Def) {
	mu.Lock()
	defer mu.Unlock()
	registry[d.Name] = &d
}

func Lookup(name string) *Def {
	mu.RLock()
	defer mu.RUnlock()
	return registry[lookupName(name)]
}

// Functions returns all registered functions sorted by name.
func Functions() []*Def {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]*Def, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
