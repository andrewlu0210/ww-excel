package model

import "testing"

func TestShiftRows(t *testing.T) {
	s := NewSheet("t")
	for r := 0; r < 6; r++ {
		s.Set(r, 0, NumberCell(float64(r)))
	}
	s.Layout.Rows[4] = Size{Px: 40}
	s.Layout.Merges = []Range{{R0: 1, C0: 0, R1: 3, C1: 1}, {R0: 2, C0: 3, R1: 2, C1: 4}}
	s.Layout.FreezeRows = 3

	s.Shift(true, 2, 2) // insert 2 rows before row 3
	if s.Get(4, 0).Num != 2 || !s.Get(2, 0).IsEmpty() || s.Get(1, 0).Num != 1 {
		t.Fatalf("insert moved cells wrong: %v %v %v", s.Get(1, 0), s.Get(2, 0), s.Get(4, 0))
	}
	if s.Layout.Rows[6].Px != 40 || s.Layout.FreezeRows != 5 {
		t.Fatalf("layout after insert: %+v", s.Layout)
	}
	if m := s.Layout.Merges[0]; m.R0 != 1 || m.R1 != 5 {
		t.Fatalf("merge spanning the insertion should grow: %+v", m)
	}

	removed := s.Shift(true, 4, -3) // delete rows 5..7 (values 2, 3, 4)
	if len(removed) != 3 || s.Get(4, 0).Num != 5 {
		t.Fatalf("delete: removed %d, row 5 = %v", len(removed), s.Get(4, 0))
	}
	if r, _ := s.UsedRange(); r != 5 {
		t.Fatalf("used rows = %d, want 5", r)
	}
}

func TestShiftColsDropsCollapsedMerge(t *testing.T) {
	s := NewSheet("t")
	s.Layout.Merges = []Range{{R0: 0, C0: 2, R1: 0, C1: 3}}
	s.Layout.ColStyles[5] = 7
	s.Shift(false, 3, -1)
	if len(s.Layout.Merges) != 0 {
		t.Fatalf("merge shrunk to one cell should be removed: %+v", s.Layout.Merges)
	}
	if s.Layout.ColStyles[4] != 7 {
		t.Fatalf("column styles: %+v", s.Layout.ColStyles)
	}
}
