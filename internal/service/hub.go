package service

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ww-excel/internal/document"
)

// Sessions resolves the session of the window a binding call came from.
// A *Session resolves to itself, which tests use.
type Sessions interface {
	For(ctx context.Context) *Session
}

func (s *Session) For(context.Context) *Session { return s }

// Hub tracks one session per document window.
type Hub struct {
	mu       sync.Mutex
	byWindow map[uint]*Session
	order    []uint
}

func NewHub() *Hub { return &Hub{byWindow: map[uint]*Session{}} }

func (h *Hub) Add(windowID uint, s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.byWindow[windowID] = s
	h.order = append(h.order, windowID)
}

func (h *Hub) Remove(windowID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.byWindow, windowID)
	for i, id := range h.order {
		if id == windowID {
			h.order = append(h.order[:i], h.order[i+1:]...)
			break
		}
	}
}

// For returns the calling window's session, or the newest one when the
// call did not come from a window.
func (h *Hub) For(ctx context.Context) *Session {
	h.mu.Lock()
	defer h.mu.Unlock()
	if w, ok := ctx.Value(application.WindowKey).(application.Window); ok && w != nil {
		if s := h.byWindow[w.ID()]; s != nil {
			return s
		}
	}
	if n := len(h.order); n > 0 {
		return h.byWindow[h.order[n-1]]
	}
	return nil
}

// ByPath finds the window showing a file.
func (h *Hub) ByPath(path string) (uint, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, s := range h.byWindow {
		var p string
		s.read(func(d *document.Document) { p = d.Path() })
		if p != "" && filepath.Clean(p) == filepath.Clean(path) {
			return id, true
		}
	}
	return 0, false
}

// All returns the sessions in window order.
func (h *Hub) All() []*Session {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*Session, 0, len(h.order))
	for _, id := range h.order {
		out = append(out, h.byWindow[id])
	}
	return out
}

// WindowContext is a context for calling services on behalf of a window
// from Go (e.g. its closing hook).
func WindowContext(w application.Window) context.Context {
	return context.WithValue(context.Background(), application.WindowKey, w)
}
