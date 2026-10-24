package model

import "sync"

// Font sizes are in points; colors are "RRGGBB" hex, "" meaning the
// default (automatic) color.
type Font struct {
	Name      string
	Size      float64
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	Color     string
}

// BorderLine styles follow xlsx: thin, medium, thick, dashed, dotted,
// double, hair, ... ("" = none).
type BorderLine struct {
	Style string
	Color string
}

type Borders struct {
	Left, Right, Top, Bottom BorderLine
}

// Align: H is left/center/right/fill/justify/... and V is
// top/center/bottom ("" = the default: general horizontal, bottom).
type Align struct {
	H      string
	V      string
	Wrap   bool
	Indent int
}

// Style is a complete cell format. It is comparable so identical styles
// share one ID in the style table.
type Style struct {
	Font   Font
	Fill   string // solid fill color, "" = none
	Border Borders
	Align  Align
	NumFmt string // number format code, "" = General
}

// DefaultFont is Excel's default for new workbooks.
var DefaultFont = Font{Name: "Calibri", Size: 11}

// StyleTable stores each distinct style once. ID 0 is the workbook's
// default style. IDs are never reused, so they stay valid for undo.
type StyleTable struct {
	mu    sync.RWMutex
	list  []Style
	index map[Style]uint32
}

func NewStyleTable(def Style) *StyleTable {
	return &StyleTable{list: []Style{def}, index: map[Style]uint32{def: 0}}
}

// Intern returns the ID of s, adding it if needed.
func (t *StyleTable) Intern(s Style) uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if id, ok := t.index[s]; ok {
		return id
	}
	id := uint32(len(t.list))
	t.list = append(t.list, s)
	t.index[s] = id
	return id
}

func (t *StyleTable) Get(id uint32) Style {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if int(id) >= len(t.list) {
		return t.list[0]
	}
	return t.list[id]
}

func (t *StyleTable) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.list)
}

// Since returns the styles with ID >= from, for incremental sync.
func (t *StyleTable) Since(from int) []Style {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if from >= len(t.list) {
		return nil
	}
	return append([]Style(nil), t.list[max(0, from):]...)
}

func (t *StyleTable) Default() Style { return t.Get(0) }
