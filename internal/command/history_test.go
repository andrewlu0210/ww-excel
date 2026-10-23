package command

import (
	"testing"

	"ww-excel/internal/model"
)

func set(row, col int, v float64) *SetCells {
	return &SetCells{Name: "輸入", Changes: []CellChange{{Row: row, Col: col, After: model.NumberCell(v)}}}
}

func num(wb *model.Workbook, row, col int) float64 {
	return wb.Sheets[0].Get(row, col).Num
}

func TestUndoRedo(t *testing.T) {
	wb := model.NewWorkbook()
	h := NewHistory(0)

	h.Exec(wb, set(0, 0, 1))
	h.Exec(wb, set(0, 0, 2))
	if num(wb, 0, 0) != 2 {
		t.Fatal("exec did not apply")
	}

	if _, ok := h.Undo(wb); !ok || num(wb, 0, 0) != 1 {
		t.Fatalf("undo -> %v", num(wb, 0, 0))
	}
	if _, ok := h.Undo(wb); !ok || wb.Sheets[0].Get(0, 0).Kind != model.KindEmpty {
		t.Fatal("second undo should clear the cell")
	}
	if _, ok := h.Undo(wb); ok {
		t.Fatal("undo on empty history should fail")
	}
	if _, ok := h.Redo(wb); !ok || num(wb, 0, 0) != 1 {
		t.Fatal("redo")
	}

	// A new edit drops the redo stack.
	h.Exec(wb, set(1, 1, 9))
	if h.CanRedo() {
		t.Fatal("redo stack should be cleared")
	}
}

func TestSetCellsSameCellTwice(t *testing.T) {
	wb := model.NewWorkbook()
	wb.Sheets[0].Set(0, 0, model.NumberCell(5))
	h := NewHistory(0)
	h.Exec(wb, &SetCells{Changes: []CellChange{
		{Row: 0, Col: 0, After: model.NumberCell(6)},
		{Row: 0, Col: 0, After: model.NumberCell(7)},
	}})
	h.Undo(wb)
	if num(wb, 0, 0) != 5 {
		t.Fatalf("undo restored %v, want 5", num(wb, 0, 0))
	}
}

func TestEffectBounds(t *testing.T) {
	wb := model.NewWorkbook()
	eff := NewHistory(0).Exec(wb, &SetCells{Changes: []CellChange{
		{Row: 5, Col: 2, After: model.NumberCell(1)},
		{Row: 1, Col: 7, After: model.NumberCell(1)},
	}})
	want := model.Range{R0: 1, C0: 2, R1: 5, C1: 7}
	if eff.Bounds == nil || *eff.Bounds != want || len(eff.Cells) != 2 {
		t.Fatalf("effect = %+v", eff)
	}
}

func TestDirty(t *testing.T) {
	wb := model.NewWorkbook()
	h := NewHistory(3)
	if h.Dirty() {
		t.Fatal("new history is clean")
	}

	h.Exec(wb, set(0, 0, 1))
	if !h.Dirty() {
		t.Fatal("edit makes dirty")
	}
	h.Undo(wb)
	if h.Dirty() {
		t.Fatal("undo back to saved state is clean")
	}
	h.Redo(wb)
	h.MarkSaved()
	if h.Dirty() {
		t.Fatal("saved is clean")
	}
	h.Undo(wb)
	if !h.Dirty() {
		t.Fatal("undo past save is dirty")
	}
	h.Redo(wb)
	if h.Dirty() {
		t.Fatal("redo to saved state is clean")
	}

	// Undo past the save, then branch: the saved state is unreachable.
	h.Undo(wb)
	h.Exec(wb, set(2, 2, 2))
	h.Undo(wb)
	if !h.Dirty() {
		t.Fatal("saved state discarded by branching must stay dirty")
	}
}

func TestLimitTrimsAndKeepsDirty(t *testing.T) {
	wb := model.NewWorkbook()
	h := NewHistory(2)
	h.Exec(wb, set(0, 0, 1))
	h.MarkSaved()
	h.Exec(wb, set(0, 0, 2))
	h.Exec(wb, set(0, 0, 3)) // trims the saved entry
	h.Undo(wb)
	h.Undo(wb)
	if h.CanUndo() {
		t.Fatal("limit should cap undo depth")
	}
	if !h.Dirty() {
		t.Fatal("trimmed saved state must stay dirty")
	}
}
