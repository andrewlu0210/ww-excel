package model

import "sort"

// Range is an inclusive rectangle of cells.
type Range struct {
	R0, C0, R1, C1 int
}

func (r Range) Normalize() Range {
	if r.R0 > r.R1 {
		r.R0, r.R1 = r.R1, r.R0
	}
	if r.C0 > r.C1 {
		r.C0, r.C1 = r.C1, r.C0
	}
	return r
}

func (r Range) Clamp() Range {
	r = r.Normalize()
	r.R0, r.C0 = max(r.R0, 0), max(r.C0, 0)
	r.R1, r.C1 = min(r.R1, MaxRows-1), min(r.C1, MaxCols-1)
	return r
}

func (r Range) Contains(row, col int) bool {
	return row >= r.R0 && row <= r.R1 && col >= r.C0 && col <= r.C1
}

func (r Range) CellCount() int {
	return (r.R1 - r.R0 + 1) * (r.C1 - r.C0 + 1)
}

// EachInRange calls fn for every non-empty cell inside rg. It walks only
// allocated blocks, so whole-column or whole-sheet ranges stay cheap.
// Cells are visited in row-major order.
func (s *Sheet) EachInRange(rg Range, fn func(row, col int, cell Cell)) {
	rg = rg.Clamp()
	var keys []BlockKey
	for k := range s.blocks {
		br0, bc0 := int(k.Row)*BlockRows, int(k.Col)*BlockCols
		if br0+BlockRows-1 < rg.R0 || br0 > rg.R1 || bc0+BlockCols-1 < rg.C0 || bc0 > rg.C1 {
			continue
		}
		keys = append(keys, k)
	}

	type hit struct {
		row, col int
		cell     Cell
	}
	var hits []hit
	for _, k := range keys {
		b := s.blocks[k]
		br0, bc0 := int(k.Row)*BlockRows, int(k.Col)*BlockCols
		b.Each(func(r, c int, cell Cell) {
			row, col := br0+r, bc0+c
			if rg.Contains(row, col) {
				hits = append(hits, hit{row, col, cell})
			}
		})
	}
	// Blocks side by side interleave rows, so sort the hits themselves.
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].row != hits[j].row {
			return hits[i].row < hits[j].row
		}
		return hits[i].col < hits[j].col
	})
	for _, h := range hits {
		fn(h.row, h.col, h.cell)
	}
}

func sortKeys(keys []BlockKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Row != keys[j].Row {
			return keys[i].Row < keys[j].Row
		}
		return keys[i].Col < keys[j].Col
	})
}

// SortedBlockKeys returns allocated block keys in row-major order.
func (s *Sheet) SortedBlockKeys() []BlockKey {
	keys := make([]BlockKey, 0, len(s.blocks))
	for k := range s.blocks {
		keys = append(keys, k)
	}
	sortKeys(keys)
	return keys
}

func (s *Sheet) filled(row, col int) bool {
	return s.Get(row, col).Kind != KindEmpty
}

// DataEdge implements Excel's ⌘+arrow jump from (row, col) in direction
// (dr, dc), where exactly one of dr/dc is ±1:
//   - inside a run of data, move to the last filled cell of the run;
//   - otherwise move to the next filled cell;
//   - if there is none, move to the sheet edge.
func (s *Sheet) DataEdge(row, col, dr, dc int) (int, int) {
	inBounds := func(r, c int) bool { return r >= 0 && r < MaxRows && c >= 0 && c < MaxCols }
	nr, nc := row+dr, col+dc
	if !inBounds(nr, nc) {
		return row, col
	}

	if s.filled(row, col) && s.filled(nr, nc) {
		for inBounds(nr+dr, nc+dc) && s.filled(nr+dr, nc+dc) {
			nr, nc = nr+dr, nc+dc
		}
		return nr, nc
	}

	// Scan for the next filled cell, skipping unallocated blocks.
	for inBounds(nr, nc) {
		if s.blocks[KeyOf(nr, nc)] == nil {
			nr, nc = s.skipBlock(nr, nc, dr, dc)
			continue
		}
		if s.filled(nr, nc) {
			return nr, nc
		}
		nr, nc = nr+dr, nc+dc
	}
	// No data: stop at the sheet edge. Only the travel axis can be out
	// of bounds here, so clamping both lands on the edge cell.
	return clampInt(nr, 0, MaxRows-1), clampInt(nc, 0, MaxCols-1)
}

// skipBlock moves (row, col) to the first cell of the next block in the
// direction of travel.
func (s *Sheet) skipBlock(row, col, dr, dc int) (int, int) {
	switch {
	case dr > 0:
		return (row/BlockRows + 1) * BlockRows, col
	case dr < 0:
		return (row/BlockRows)*BlockRows - 1, col
	case dc > 0:
		return row, (col/BlockCols + 1) * BlockCols
	default:
		return row, (col/BlockCols)*BlockCols - 1
	}
}

func clampInt(v, lo, hi int) int { return max(lo, min(v, hi)) }
