// Package command implements undoable edits. Every workbook mutation goes
// through a Command so it can be undone; a multi-step operation is a
// single Command and therefore a single undo step.
package command

import "ww-excel/internal/model"

// CellRef identifies a cell on a sheet.
type CellRef struct {
	Row, Col int
}

// Effect describes what a Do/Undo changed, for diff events and for
// selecting the affected area after undo/redo.
type Effect struct {
	Sheet int
	Cells []CellRef
	// Bounds of the edit; nil when nothing changed.
	Bounds *model.Range
}

type Command interface {
	Do(wb *model.Workbook) Effect
	Undo(wb *model.Workbook) Effect
	Label() string
}

// CellChange is one cell's new value; the old value is captured on Do.
type CellChange struct {
	Row, Col int
	After    model.Cell
	before   model.Cell
}

// SetCells writes a batch of cells as one undo step.
type SetCells struct {
	Sheet   int
	Name    string
	Changes []CellChange
	// Bounds is reported to the frontend as the affected area. When nil
	// it is computed from the changes.
	Area *model.Range
}

func (s *SetCells) Label() string { return s.Name }

func (s *SetCells) Do(wb *model.Workbook) Effect {
	sh := wb.Sheets[s.Sheet]
	for i := range s.Changes {
		ch := &s.Changes[i]
		ch.before = sh.Get(ch.Row, ch.Col)
		sh.Set(ch.Row, ch.Col, ch.After)
	}
	return s.effect()
}

func (s *SetCells) Undo(wb *model.Workbook) Effect {
	sh := wb.Sheets[s.Sheet]
	// Reverse order so repeated writes to one cell restore correctly.
	for i := len(s.Changes) - 1; i >= 0; i-- {
		ch := s.Changes[i]
		sh.Set(ch.Row, ch.Col, ch.before)
	}
	return s.effect()
}

func (s *SetCells) effect() Effect {
	e := Effect{Sheet: s.Sheet, Cells: make([]CellRef, len(s.Changes)), Bounds: s.Area}
	for i, ch := range s.Changes {
		e.Cells[i] = CellRef{ch.Row, ch.Col}
		if s.Area == nil {
			e.Bounds = grow(e.Bounds, ch.Row, ch.Col)
		}
	}
	return e
}

func grow(r *model.Range, row, col int) *model.Range {
	if r == nil {
		return &model.Range{R0: row, C0: col, R1: row, C1: col}
	}
	r.R0, r.C0 = min(r.R0, row), min(r.C0, col)
	r.R1, r.C1 = max(r.R1, row), max(r.C1, col)
	return r
}
