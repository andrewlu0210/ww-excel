// Package service is the thin Wails-bound layer over internal packages.
// Keep logic out of here so it can be tested without Wails.
package service

import (
	"sort"
	"sync"

	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

// Event names pushed from Go to the frontend.
const (
	EventCellsChanged  = "cells:changed"
	EventLayoutChanged = "layout:changed"
	EventSheetsChanged = "sheets:changed"
	EventDocState      = "doc:state"
	EventDocReset      = "doc:reset"
	EventMenuAction    = "menu:action"
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
	// N: number (right-aligned). X: logical or error (centered).
	N bool `json:"n,omitempty"`
	X bool `json:"x,omitempty"`
	// St is the style ID; Fc a color from the number format ([Red]).
	St uint32 `json:"st,omitempty"`
	Fc string `json:"fc,omitempty"`
}

// CellsChanged is emitted once per edit, undo or redo for each sheet
// it touched (formulas on other sheets may recalculate).
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
	// Styles created since the last event.
	Styles []StyleDTO `json:"styles"`
}

// LayoutChanged tells the frontend to refetch a sheet's layout.
type LayoutChanged struct {
	Sheet int `json:"sheet"`
}

type DocState struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Dirty     bool   `json:"dirty"`
	CanUndo   bool   `json:"canUndo"`
	CanRedo   bool   `json:"canRedo"`
	UndoLabel string `json:"undoLabel"`
	RedoLabel string `json:"redoLabel"`
	// Circular is the first cell in a circular reference, or "".
	Circular string `json:"circular"`
	// Sheets are the sheet names in order.
	Sheets []string `json:"sheets"`
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

	// Number of styles already sent with diff events.
	stylesSent int
}

func NewSession(doc *document.Document, emit Emitter) *Session {
	if emit == nil {
		emit = func(string, any) {}
	}
	// Existing styles reach the frontend through GetBlocks/Layout; diff
	// events only carry styles created later.
	return &Session{doc: doc, emit: emit, stylesSent: doc.Workbook().StyleTable().Len()}
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
		Circular:  s.doc.Circular(),
		Sheets:    s.sheetNamesLocked(),
	}
}

func (s *Session) sheetNamesLocked() []string {
	names := make([]string, len(s.doc.Workbook().Sheets))
	for i, sh := range s.doc.Workbook().Sheets {
		names[i] = sh.Name
	}
	return names
}

// change runs a mutation under the write lock, then emits the events
// for it: one diff per touched sheet, layout or sheet-list changes, and
// the new document state.
func (s *Session) change(selectAfter bool, fn func(d *document.Document) (document.Change, bool)) bool {
	s.mu.Lock()
	ch, ok := fn(s.doc)
	var diffs []CellsChanged
	var state DocState
	if ok {
		diffs = s.diffsLocked(ch, selectAfter)
		state = s.stateLocked()
	}
	s.mu.Unlock()

	if !ok {
		return false
	}
	if ch.Structure {
		s.emit(EventSheetsChanged, state)
	}
	if ch.LayoutChanged {
		s.emit(EventLayoutChanged, LayoutChanged{Sheet: ch.Sheet})
	}
	for _, d := range diffs {
		s.emit(EventCellsChanged, d)
	}
	s.publishState(state)
	return true
}

// cellView renders a cell for the frontend with its effective style.
func cellView(wb *model.Workbook, sh *model.Sheet, row, col int, c model.Cell) (v string, n, x bool, st uint32, fc string) {
	st = c.S
	if st == 0 {
		st = sh.Layout.StyleAt(row, col)
	}
	v, fc = display(c, wb.StyleTable().Get(st))
	return v, c.Kind == model.KindNumber, c.Kind == model.KindBool || c.Kind == model.KindError, st, fc
}

func (s *Session) diffsLocked(ch document.Change, selectAfter bool) []CellsChanged {
	type pos struct{ row, col int }
	wb := s.doc.Workbook()
	bySheet := map[int][]pos{}
	if !ch.Structure {
		bySheet[ch.Sheet] = nil
	}
	for _, c := range ch.Cells {
		bySheet[ch.Sheet] = append(bySheet[ch.Sheet], pos{c.Row, c.Col})
	}
	for sheet, cells := range ch.Others {
		for _, c := range cells {
			bySheet[sheet] = append(bySheet[sheet], pos{c.Row, c.Col})
		}
	}
	for _, k := range ch.Recalc {
		bySheet[k.Sheet] = append(bySheet[k.Sheet], pos{k.Row, k.Col})
	}
	sheets := make([]int, 0, len(bySheet))
	for sh := range bySheet {
		if sh >= 0 && sh < len(wb.Sheets) {
			sheets = append(sheets, sh)
		}
	}
	sort.Ints(sheets)

	var out []CellsChanged
	for _, si := range sheets {
		sh := wb.Sheets[si]
		diff := CellsChanged{Sheet: si, Version: s.doc.Version(), Cells: []ChangedCell{}}
		diff.UsedRows, diff.UsedCols = sh.UsedRange()
		cells := bySheet[si]
		if len(cells) > maxDiffCells || ch.Structure {
			diff.Invalidate = true
		} else {
			for _, p := range cells {
				v, n, x, st, fc := cellView(wb, sh, p.row, p.col, sh.Get(p.row, p.col))
				diff.Cells = append(diff.Cells, ChangedCell{Row: p.row, Col: p.col, V: v, N: n, X: x, St: st, Fc: fc})
			}
		}
		if selectAfter && si == ch.Sheet && ch.Bounds != nil {
			b := ch.Bounds
			diff.Select = &RangeDTO{R0: b.R0, C0: b.C0, R1: b.R1, C1: b.C1}
		}
		out = append(out, diff)
	}
	// New styles ride along with the first diff.
	if n := wb.StyleTable().Len(); n > s.stylesSent {
		if len(out) == 0 {
			out = append(out, CellsChanged{Sheet: ch.Sheet, Version: s.doc.Version(), Cells: []ChangedCell{}})
		}
		out[0].Styles = stylesSince(wb, s.stylesSent)
		s.stylesSent = n
	}
	for i := range out {
		if out[i].Styles == nil {
			out[i].Styles = []StyleDTO{}
		}
	}
	return out
}

// replace swaps in a new document and tells the frontend to reload.
func (s *Session) replace(doc *document.Document) {
	s.mu.Lock()
	old := s.doc
	s.doc = doc
	s.stylesSent = doc.Workbook().StyleTable().Len()
	state := s.stateLocked()
	s.mu.Unlock()
	old.Close()

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
