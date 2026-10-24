package command

import (
	"errors"
	"testing"

	"ww-excel/internal/model"
)

func TestEnterTextNoop(t *testing.T) {
	wb := model.NewWorkbook()
	wb.Sheets[0].Set(0, 0, model.NumberCell(5))
	if EnterText(wb, 0, 0, 0, "5") != nil {
		t.Fatal("same value should be a no-op")
	}
	if EnterText(wb, 0, 0, 0, "'5") == nil {
		t.Fatal("text '5' differs from number 5")
	}
}

func TestFillRanges(t *testing.T) {
	wb := model.NewWorkbook()
	h := NewHistory(0)
	seven := func(int, int) model.Cell { return model.NumberCell(7) }
	cmd, err := FillRanges(wb, 0, []model.Range{{R0: 0, C0: 0, R1: 2, C1: 1}, {R0: 10, C0: 10, R1: 10, C1: 10}}, seven)
	if err != nil {
		t.Fatal(err)
	}
	eff := h.Exec(wb, cmd)
	if len(eff.Cells) != 7 {
		t.Fatalf("filled %d cells, want 7", len(eff.Cells))
	}
	if want := (model.Range{R0: 0, C0: 0, R1: 10, C1: 10}); *eff.Bounds != want {
		t.Fatalf("bounds = %+v", *eff.Bounds)
	}
	if wb.Sheets[0].Get(2, 1) != model.NumberCell(7) {
		t.Fatal("fill not applied")
	}

	_, err = FillRanges(wb, 0, []model.Range{{R0: 0, C0: 0, R1: model.MaxRows - 1, C1: 1}}, seven)
	if !errors.Is(err, ErrFillTooLarge) {
		t.Fatalf("err = %v, want ErrFillTooLarge", err)
	}
}

func TestClearRangesWholeColumn(t *testing.T) {
	wb := model.NewWorkbook()
	s := wb.Sheets[0]
	s.Set(0, 0, model.NumberCell(1))
	s.Set(900_000, 0, model.NumberCell(2))
	s.Set(5, 1, model.NumberCell(3))
	h := NewHistory(0)

	col := model.Range{R0: 0, C0: 0, R1: model.MaxRows - 1, C1: 0}
	// Overlapping ranges must not record a cell twice.
	cmd := ClearRanges(wb, 0, []model.Range{col, {R0: 0, C0: 0, R1: 0, C1: 0}}, ClearContents)
	if got := len(cmd.(*SetCells).Changes); got != 2 {
		t.Fatalf("changes = %d, want 2", got)
	}
	h.Exec(wb, cmd)
	if s.Get(900_000, 0).Kind != model.KindEmpty || s.Get(5, 1).Kind == model.KindEmpty {
		t.Fatal("clear touched wrong cells")
	}
	h.Undo(wb)
	if s.Get(900_000, 0) != model.NumberCell(2) {
		t.Fatal("undo did not restore")
	}
	if ClearRanges(wb, 0, []model.Range{{R0: 50, C0: 50, R1: 60, C1: 60}}, ClearContents) != nil {
		t.Fatal("clearing empty range should be nil")
	}
}
