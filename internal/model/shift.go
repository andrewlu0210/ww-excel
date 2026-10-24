package model

// Structural edits: inserting and deleting whole rows or columns. These
// move cells and layout only; formulas are adjusted by the caller (the
// document), which owns the formula engine.

// Remap rebuilds the sheet storage, moving each cell to the position fn
// returns, or dropping it when keep is false. The used range is
// recomputed exactly.
func (s *Sheet) Remap(fn func(row, col int, cell Cell) (newRow, newCol int, keep bool)) {
	old := s.blocks
	s.blocks = make(map[BlockKey]*Block, len(old))
	s.usedRows, s.usedCols = 0, 0
	for k, b := range old {
		r0, c0 := int(k.Row)*BlockRows, int(k.Col)*BlockCols
		b.Each(func(r, c int, cell Cell) {
			nr, nc, keep := fn(r0+r, c0+c, cell)
			if keep && nr >= 0 && nr < MaxRows && nc >= 0 && nc < MaxCols {
				s.Set(nr, nc, cell)
			}
		})
	}
}

// Shift inserts (count > 0) or deletes (count < 0) rows or columns at
// index at, moving cells and layout. Deleted cells are returned in their
// original positions (for undo).
func (s *Sheet) Shift(rows bool, at, count int) []CellAt {
	var removed []CellAt
	s.Remap(func(row, col int, cell Cell) (int, int, bool) {
		i := col
		if rows {
			i = row
		}
		ni, keep := shiftIndex(i, at, count)
		if !keep {
			removed = append(removed, CellAt{Row: row, Col: col, Cell: cell})
			return 0, 0, false
		}
		if rows {
			return ni, col, true
		}
		return row, ni, true
	})
	s.Layout.shift(rows, at, count)
	return removed
}

// CellAt is a cell with its position.
type CellAt struct {
	Row, Col int
	Cell     Cell
}

// shiftIndex maps one index through an insertion or deletion.
func shiftIndex(i, at, count int) (int, bool) {
	if count > 0 {
		if i >= at {
			return i + count, true
		}
		return i, true
	}
	n := -count
	switch {
	case i < at:
		return i, true
	case i < at+n:
		return 0, false
	}
	return i - n, true
}

func shiftKeys[V any](m map[int]V, at, count int) map[int]V {
	out := make(map[int]V, len(m))
	for i, v := range m {
		if ni, keep := shiftIndex(i, at, count); keep {
			out[ni] = v
		}
	}
	return out
}

// Shift moves sizes, row/column styles, merges and frozen panes as rows
// or columns are inserted or deleted.
func (l *Layout) Shift(rows bool, at, count int) { l.shift(rows, at, count) }

func (l *Layout) shift(rows bool, at, count int) {
	if rows {
		l.Rows = shiftKeys(l.Rows, at, count)
		l.RowStyles = shiftKeys(l.RowStyles, at, count)
	} else {
		l.Cols = shiftKeys(l.Cols, at, count)
		l.ColStyles = shiftKeys(l.ColStyles, at, count)
	}
	var merges []Range
	for _, m := range l.Merges {
		lo, hi := m.C0, m.C1
		if rows {
			lo, hi = m.R0, m.R1
		}
		limit := MaxCols
		if rows {
			limit = MaxRows
		}
		nlo, nhi, ok := ShiftSpan(lo, hi, at, count, limit)
		if !ok {
			continue
		}
		if rows {
			m.R0, m.R1 = nlo, nhi
		} else {
			m.C0, m.C1 = nlo, nhi
		}
		if m.R0 == m.R1 && m.C0 == m.C1 {
			continue // shrunk to one cell: no longer a merge
		}
		merges = append(merges, m)
	}
	l.Merges = merges
	if f := l.Filter; f != nil {
		r := f.Range
		lo, hi := r.C0, r.C1
		limit := MaxCols
		if rows {
			lo, hi, limit = r.R0, r.R1, MaxRows
		}
		nlo, nhi, ok := ShiftSpan(lo, hi, at, count, limit)
		if !ok {
			l.Filter = nil
		} else {
			f = f.Clone()
			if rows {
				f.Range.R0, f.Range.R1 = nlo, nhi
			} else {
				f.Range.C0, f.Range.C1 = nlo, nhi
				f.Cols = shiftKeys(f.Cols, at, count)
			}
			l.Filter = f
		}
	}
	if rows {
		l.FreezeRows = shiftFreeze(l.FreezeRows, at, count)
	} else {
		l.FreezeCols = shiftFreeze(l.FreezeCols, at, count)
	}
}

func shiftFreeze(n, at, count int) int {
	if n == 0 || at >= n {
		return n
	}
	if count > 0 {
		return n + count
	}
	return max(at, n+count)
}

// ShiftSpan maps an inclusive span [lo, hi] through an insertion or
// deletion, as Excel adjusts ranges: spans grow when rows are inserted
// inside them and shrink when rows inside are deleted; ok is false when
// the whole span is deleted.
func ShiftSpan(lo, hi, at, count, limit int) (int, int, bool) {
	if count > 0 {
		if lo >= at {
			lo += count
		}
		if hi >= at {
			hi += count
		}
		if lo >= limit {
			return 0, 0, false
		}
		return lo, min(hi, limit-1), true
	}
	n := -count
	end := at + n - 1
	switch {
	case hi < at:
		return lo, hi, true
	case lo > end:
		return lo - n, hi - n, true
	}
	newLo := lo
	if lo >= at {
		newLo = at
	}
	newHi := hi - n
	if hi <= end {
		newHi = at - 1
	}
	if newHi < newLo {
		return 0, 0, false
	}
	return newLo, newHi, true
}

// ShiftIndex maps one row or column index through an insertion or
// deletion; keep is false for deleted indices.
func ShiftIndex(i, at, count int) (int, bool) { return shiftIndex(i, at, count) }
