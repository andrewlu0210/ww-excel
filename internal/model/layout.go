package model

import "sort"

// Default sizes in pixels at 100% zoom, matching Excel: rows are 15pt
// (20px) and columns 8.43 characters (64px).
const (
	DefaultRowHeight = 20
	DefaultColWidth  = 64
)

// Size is a custom row height or column width. Hidden rows/columns keep
// their size so unhiding restores it.
type Size struct {
	Px     float64
	Hidden bool
}

// Layout holds the per-sheet geometry and whole-row/column formats.
type Layout struct {
	DefaultRow float64
	DefaultCol float64
	Rows       map[int]Size
	Cols       map[int]Size
	// Whole row/column styles: cells without their own style inherit
	// them (row style wins over column style, as in Excel).
	RowStyles map[int]uint32
	ColStyles map[int]uint32
	Merges    []Range
	// Frozen panes: the first FreezeRows rows and FreezeCols columns
	// stay on screen while scrolling (0 = none).
	FreezeRows int
	FreezeCols int
	// Filter is the sheet's AutoFilter, or nil.
	Filter *AutoFilter
}

// AutoFilter is a filtered range: its first row holds the dropdown
// buttons; rows below that fail a column's criteria are hidden.
type AutoFilter struct {
	Range Range
	// Cols maps a column index to the display values it shows; columns
	// without an entry show everything.
	Cols map[int][]string
}

func (f *AutoFilter) Clone() *AutoFilter {
	if f == nil {
		return nil
	}
	out := &AutoFilter{Range: f.Range, Cols: make(map[int][]string, len(f.Cols))}
	for c, v := range f.Cols {
		out.Cols[c] = append([]string(nil), v...)
	}
	return out
}

func NewLayout() Layout {
	return Layout{
		DefaultRow: DefaultRowHeight,
		DefaultCol: DefaultColWidth,
		Rows:       map[int]Size{},
		Cols:       map[int]Size{},
		RowStyles:  map[int]uint32{},
		ColStyles:  map[int]uint32{},
	}
}

// Clone deep-copies the layout (for undo snapshots).
func (l Layout) Clone() Layout {
	out := l
	out.Rows = cloneMap(l.Rows)
	out.Cols = cloneMap(l.Cols)
	out.RowStyles = cloneMap(l.RowStyles)
	out.ColStyles = cloneMap(l.ColStyles)
	out.Merges = append([]Range(nil), l.Merges...)
	out.Filter = l.Filter.Clone()
	return out
}

func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	out := make(map[K]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// RowHeight is the visible height of a row (0 when hidden).
func (l *Layout) RowHeight(row int) float64 {
	if s, ok := l.Rows[row]; ok {
		if s.Hidden {
			return 0
		}
		return s.Px
	}
	return l.DefaultRow
}

func (l *Layout) ColWidth(col int) float64 {
	if s, ok := l.Cols[col]; ok {
		if s.Hidden {
			return 0
		}
		return s.Px
	}
	return l.DefaultCol
}

// StyleAt is the inherited style of a position without its own style.
func (l *Layout) StyleAt(row, col int) uint32 {
	if s, ok := l.RowStyles[row]; ok {
		return s
	}
	return l.ColStyles[col]
}

// MergeAt returns the merged area containing (row, col).
func (l *Layout) MergeAt(row, col int) (Range, bool) {
	for _, m := range l.Merges {
		if m.Contains(row, col) {
			return m, true
		}
	}
	return Range{}, false
}

// SortMerges keeps merges in row-major order of their top-left cell.
func (l *Layout) SortMerges() {
	sort.Slice(l.Merges, func(i, j int) bool {
		a, b := l.Merges[i], l.Merges[j]
		if a.R0 != b.R0 {
			return a.R0 < b.R0
		}
		return a.C0 < b.C0
	})
}

// Overlaps reports whether two ranges share any cell.
func (r Range) Overlaps(o Range) bool {
	return r.R0 <= o.R1 && o.R0 <= r.R1 && r.C0 <= o.C1 && o.C0 <= r.C1
}
