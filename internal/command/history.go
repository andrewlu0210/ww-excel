package command

import "ww-excel/internal/model"

// DefaultLimit matches Excel's default of 100 undo levels.
const DefaultLimit = 100

type entry struct {
	cmd Command
	seq uint64
}

// History is the undo/redo stack. It also tracks whether the workbook
// differs from the last saved state.
type History struct {
	done   []entry
	undone []entry
	limit  int
	seq    uint64
	// savedSeq is the seq of the top of done at the last save; 0 means
	// the empty history. Once the saved entry is trimmed or discarded,
	// savedLost keeps the document dirty.
	savedSeq  uint64
	savedLost bool
}

func NewHistory(limit int) *History {
	if limit <= 0 {
		limit = DefaultLimit
	}
	return &History{limit: limit}
}

func (h *History) Exec(wb *model.Workbook, cmd Command) Effect {
	eff := cmd.Do(wb)
	h.seq++
	h.done = append(h.done, entry{cmd, h.seq})
	// A new edit invalidates redo; if the saved state was there, it is
	// no longer reachable.
	for _, e := range h.undone {
		if e.seq == h.savedSeq {
			h.savedLost = true
		}
	}
	h.undone = nil
	if len(h.done) > h.limit {
		if h.done[0].seq == h.savedSeq {
			h.savedLost = true
		}
		h.done = h.done[1:]
	}
	return eff
}

func (h *History) Undo(wb *model.Workbook) (Effect, bool) {
	if len(h.done) == 0 {
		return Effect{}, false
	}
	e := h.done[len(h.done)-1]
	h.done = h.done[:len(h.done)-1]
	h.undone = append(h.undone, e)
	return e.cmd.Undo(wb), true
}

func (h *History) Redo(wb *model.Workbook) (Effect, bool) {
	if len(h.undone) == 0 {
		return Effect{}, false
	}
	e := h.undone[len(h.undone)-1]
	h.undone = h.undone[:len(h.undone)-1]
	h.done = append(h.done, e)
	return e.cmd.Do(wb), true
}

func (h *History) CanUndo() bool { return len(h.done) > 0 }
func (h *History) CanRedo() bool { return len(h.undone) > 0 }

func (h *History) UndoLabel() string {
	if len(h.done) == 0 {
		return ""
	}
	return h.done[len(h.done)-1].cmd.Label()
}

func (h *History) RedoLabel() string {
	if len(h.undone) == 0 {
		return ""
	}
	return h.undone[len(h.undone)-1].cmd.Label()
}

func (h *History) top() uint64 {
	if len(h.done) == 0 {
		return 0
	}
	return h.done[len(h.done)-1].seq
}

// MarkSaved records the current state as saved.
// MarkUnsaved makes the document dirty until the next save, e.g. for a
// workbook recovered after a crash.
func (h *History) MarkUnsaved() { h.savedLost = true }

func (h *History) MarkSaved() {
	h.savedLost = false
	h.savedSeq = 0
	if len(h.done) > 0 {
		h.savedSeq = h.done[len(h.done)-1].seq
	}
}

// Dirty reports whether the workbook differs from the last saved state.
func (h *History) Dirty() bool {
	if h.savedLost {
		return true
	}
	return h.top() != h.savedSeq
}
