package model

import "testing"

func TestSheetSetGet(t *testing.T) {
	tests := []struct {
		name     string
		row, col int
		cell     Cell
	}{
		{"origin", 0, 0, NumberCell(1.5)},
		{"block edge", BlockRows - 1, BlockCols - 1, StringCell("edge")},
		{"next block", BlockRows, BlockCols, StringCell("next")},
		{"last cell", MaxRows - 1, MaxCols - 1, NumberCell(-3)},
	}
	s := NewSheet("t")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.Set(tt.row, tt.col, tt.cell)
			if got := s.Get(tt.row, tt.col); got != tt.cell {
				t.Fatalf("Get(%d,%d) = %+v, want %+v", tt.row, tt.col, got, tt.cell)
			}
		})
	}
	if got := s.BlockCount(); got != 3 {
		t.Fatalf("BlockCount = %d, want 3", got)
	}
}

func TestSheetClearFreesBlock(t *testing.T) {
	s := NewSheet("t")
	s.Set(10, 10, NumberCell(1))
	s.Set(11, 10, NumberCell(2))
	s.Set(10, 10, Cell{})
	if s.BlockCount() != 1 {
		t.Fatalf("block freed too early")
	}
	s.Set(11, 10, Cell{})
	if s.BlockCount() != 0 {
		t.Fatalf("empty block not freed")
	}
	// Setting an empty cell must not allocate.
	s.Set(500, 500, Cell{})
	if s.BlockCount() != 0 {
		t.Fatalf("empty Set allocated a block")
	}
}

func TestUsedRange(t *testing.T) {
	s := NewSheet("t")
	s.Set(99, 4, StringCell("x"))
	s.Set(3, 7, StringCell("y"))
	if r, c := s.UsedRange(); r != 100 || c != 8 {
		t.Fatalf("UsedRange = (%d,%d), want (100,8)", r, c)
	}
}

func TestDisplay(t *testing.T) {
	tests := []struct {
		cell Cell
		want string
	}{
		{Cell{}, ""},
		{NumberCell(1234.5), "1234.5"},
		{NumberCell(1e21), "1E+21"},
		{NumberCell(0.1 + 0.2), "0.3"},
		{StringCell("台北"), "台北"},
		{BoolCell(true), "TRUE"},
		{ErrorCell(ErrDiv0), "#DIV/0!"},
	}
	for _, tt := range tests {
		if got := tt.cell.Display(); got != tt.want {
			t.Errorf("Display(%+v) = %q, want %q", tt.cell, got, tt.want)
		}
	}
}

func BenchmarkFillFake100k(b *testing.B) {
	for b.Loop() {
		FillFake(NewSheet("b"), 20_000, 5, 1)
	}
}
