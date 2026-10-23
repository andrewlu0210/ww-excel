package model

import "testing"

func TestDataEdge(t *testing.T) {
	s := NewSheet("t")
	// Column A: rows 0-4 filled, gap, rows 10-12 filled, then a far cell.
	for _, r := range []int{0, 1, 2, 3, 4, 10, 11, 12, 500_000} {
		s.Set(r, 0, NumberCell(1))
	}
	s.Set(0, 5, NumberCell(1))

	tests := []struct {
		name         string
		row, col     int
		dr, dc       int
		wantR, wantC int
	}{
		{"run to end", 0, 0, 1, 0, 4, 0},
		{"end of run to next run", 4, 0, 1, 0, 10, 0},
		{"gap to next run", 6, 0, 1, 0, 10, 0},
		{"across empty blocks", 12, 0, 1, 0, 500_000, 0},
		{"past last data to edge", 500_000, 0, 1, 0, MaxRows - 1, 0},
		{"up to run start", 12, 0, -1, 0, 10, 0},
		{"up from top stays", 0, 0, -1, 0, 0, 0},
		{"right to next filled", 0, 0, 0, 1, 0, 5},
		{"right past data to edge", 0, 5, 0, 1, 0, MaxCols - 1},
		{"left from empty to edge", 3, 200, 0, -1, 3, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, c := s.DataEdge(tt.row, tt.col, tt.dr, tt.dc)
			if r != tt.wantR || c != tt.wantC {
				t.Fatalf("DataEdge(%d,%d,%d,%d) = (%d,%d), want (%d,%d)",
					tt.row, tt.col, tt.dr, tt.dc, r, c, tt.wantR, tt.wantC)
			}
		})
	}
}

func TestEachInRange(t *testing.T) {
	s := NewSheet("t")
	s.Set(0, 40, StringCell("b"))  // block (0,1)
	s.Set(1, 0, StringCell("c"))   // block (0,0)
	s.Set(0, 0, StringCell("a"))   // block (0,0)
	s.Set(100, 0, StringCell("x")) // outside the range

	var got []string
	s.EachInRange(Range{R0: 0, C0: 0, R1: 50, C1: 100}, func(_, _ int, c Cell) {
		got = append(got, c.Str)
	})
	if want := "a b c"; joinStr(got) != want {
		t.Fatalf("order = %q, want %q", joinStr(got), want)
	}

	n := 0
	s.EachInRange(Range{R0: 0, C0: 0, R1: MaxRows - 1, C1: 0}, func(int, int, Cell) { n++ })
	if n != 3 {
		t.Fatalf("whole column visited %d cells, want 3", n)
	}
}

func joinStr(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}
