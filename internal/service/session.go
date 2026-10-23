// Package service is the thin Wails-bound layer over internal packages.
// Keep logic out of here so it can be tested without Wails.
package service

import (
	"sync"

	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

// Event names pushed from Go to the frontend.
const (
	EventCellsChanged = "cells:changed"
	EventDocState     = "doc:state"
	EventDocReset     = "doc:reset"
	EventMenuAction   = "menu:action"
)

// Diffs larger than this are sent as an invalidation instead of a cell
// list; the frontend then refetches the visible blocks.
const maxDiffCells = 5000

type RangeDTO struct {
	R0 int `json:"r0"`
	C0 int `json:"c0"`
	R1 int `json:"r1"`
	C1 int `json:"c1"`
}

func (r RangeDTO) model() model.Range { return model.Range{R0: r.R0, C0: r.C0, R1: r.R1, C1: r.C1} }

type ChangedCell struct {
	Row int    `json:"row"`
	Col int    `json:"col"`
	V   string `json:"v"`
	N   bool   `json:"n,omitempty"`
}

// CellsChanged is the single event emitted per edit, undo or redo.
type CellsChanged struct {
	Sheet   int           `json:"sheet"`
	Version uint64        `json:"version"`
	Cells   []ChangedCell `json:"cells"`
	// Invalidate means the change was too large to list; drop cached
	// blocks and refetch.
	Invalidate bool `json:"invalidate,omitempty"`
	UsedRows   int  `json:"usedRows"`
	UsedCols   int  `json:"usedCols"`
	// Select is set for undo/redo: Excel selects the affected area.
	Select *RangeDTO `json:"select,omitempty"`
}

type DocState struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Dirty     bool   `json:"dirty"`
	CanUndo   bool   `json:"canUndo"`
	CanRedo   bool   `json:"canRedo"`
	UndoLabel string `json:"undoLabel"`
	RedoLabel string `json:"redoLabel"`
}

type Emitter func(name string, data any)

// Session owns the open document. Both services share it.
type Session struct {
	mu   sync.RWMutex
	doc  *document.Document
	emit Emitter

	// OnState is called after every state change, e.g. to update the
	// window title and menu labels. Optional.
	OnState func(DocState)
}

func NewSession(doc *document.Document, emit Emitter) *Session {
	if emit == nil {
		emit = func(string, any) {}
	}
	return &Session{doc: doc, emit: emit}
}

func (s *Session) read(fn func(d *document.Document)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.doc)
}

func (s *Session) State() DocState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stateLocked()
}

func (s *Session) stateLocked() DocState {
	h := s.doc.History()
	return DocState{
		Name:      s.doc.Name(),
		Path:      s.doc.Path(),
		Dirty:     s.doc.Dirty(),
		CanUndo:   h.CanUndo(),
		CanRedo:   h.CanRedo(),
		UndoLabel: h.UndoLabel(),
		RedoLabel: h.RedoLabel(),
	}
}

// change runs a mutation under the write lock, then emits one diff event
// and the new document state.
func (s *Session) change(selectAfter bool, fn func(d *document.Document) (command.Effect, bool)) bool {
	s.mu.Lock()
	eff, ok := fn(s.doc)
	var diff CellsChanged
	var state DocState
	if ok {
		diff = s.diffLocked(eff, selectAfter)
		state = s.stateLocked()
	}
	s.mu.Unlock()

	if ok {
		s.emit(EventCellsChanged, diff)
		s.publishState(state)
	}
	return ok
}

func (s *Session) diffLocked(eff command.Effect, selectAfter bool) CellsChanged {
	sh := s.doc.Workbook().Sheets[eff.Sheet]
	diff := CellsChanged{Sheet: eff.Sheet, Version: s.doc.Version(), Cells: []ChangedCell{}}
	diff.UsedRows, diff.UsedCols = sh.UsedRange()
	if len(eff.Cells) > maxDiffCells {
		diff.Invalidate = true
	} else {
		for _, ref := range eff.Cells {
			c := sh.Get(ref.Row, ref.Col)
			diff.Cells = append(diff.Cells, ChangedCell{
				Row: ref.Row, Col: ref.Col, V: c.Display(), N: c.Kind == model.KindNumber,
			})
		}
	}
	if selectAfter && eff.Bounds != nil {
		b := eff.Bounds
		diff.Select = &RangeDTO{R0: b.R0, C0: b.C0, R1: b.R1, C1: b.C1}
	}
	return diff
}

// replace swaps in a new document and tells the frontend to reload.
func (s *Session) replace(doc *document.Document) {
	s.mu.Lock()
	s.doc = doc
	state := s.stateLocked()
	s.mu.Unlock()

	s.emit(EventDocReset, state)
	s.publishState(state)
}

func (s *Session) publishState(state DocState) {
	s.emit(EventDocState, state)
	if s.OnState != nil {
		s.OnState(state)
	}
}

// PublishState re-sends the current state, e.g. after a save.
func (s *Session) PublishState() {
	s.publishState(s.State())
}

func (s *Session) Dirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc.Dirty()
}
