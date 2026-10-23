package service

import (
	"testing"

	"ww-excel/internal/model"
)

func newTestService() *SheetService {
	wb := model.NewWorkbook()
	model.FillFake(wb.Sheets[0], 20_000, 5, 1)
	return NewSheetService(wb)
}

func TestGetBlocks(t *testing.T) {
	s := newTestService()
	got := s.GetBlocks(0, []BlockRef{{0, 0}, {5000, 5000}, {0, 1}})
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// Block (0,0) holds rows 0..63 x cols 0..4.
	if n := len(got[0].Cells); n != model.BlockRows*5 {
		t.Fatalf("block(0,0) cells = %d, want %d", n, model.BlockRows*5)
	}
	if c := got[0].Cells[0]; c.V != "Item" || c.N {
		t.Fatalf("first cell = %+v", c)
	}
	if len(got[1].Cells) != 0 || got[1].Cells == nil {
		t.Fatalf("empty block should have non-nil empty cells: %+v", got[1].Cells)
	}
}

func TestGetBlocksBadSheet(t *testing.T) {
	s := newTestService()
	got := s.GetBlocks(9, []BlockRef{{0, 0}})
	if len(got) != 1 || len(got[0].Cells) != 0 {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestInfo(t *testing.T) {
	info := newTestService().Info(0)
	if info.Rows != model.MaxRows || info.Cols != model.MaxCols {
		t.Fatalf("dims = %dx%d", info.Rows, info.Cols)
	}
	if info.UsedRows != model.MaxRows || info.UsedCols != model.MaxCols {
		t.Fatalf("used = %dx%d (edge markers should extend it)", info.UsedRows, info.UsedCols)
	}
}

// A typical viewport fetch: ~40 rows x ~15 cols plus prefetch ring
// spans about 3x3 blocks.
func BenchmarkGetBlocksViewport(b *testing.B) {
	s := newTestService()
	refs := make([]BlockRef, 0, 9)
	for r := 10; r < 13; r++ {
		for c := 0; c < 3; c++ {
			refs = append(refs, BlockRef{r, c})
		}
	}
	for b.Loop() {
		s.GetBlocks(0, refs)
	}
}
