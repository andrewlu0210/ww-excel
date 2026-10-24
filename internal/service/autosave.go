package service

import (
	"log"
	"sync"
	"time"

	"ww-excel/internal/recovery"
)

type snapshot struct {
	dirty       bool
	data        []byte
	version     uint64
	title, path string
}

// Autosaver periodically writes snapshots of unsaved documents to the
// recovery store (Excel's AutoRecover), and removes them once the
// document is saved or closed.
type Autosaver struct {
	Store *recovery.Store
	hub   *Hub

	mu sync.Mutex
	// Version of each session's last snapshot; absent when it has none.
	saved map[*Session]uint64
}

func NewAutosaver(store *recovery.Store, hub *Hub) *Autosaver {
	return &Autosaver{Store: store, hub: hub, saved: map[*Session]uint64{}}
}

// Tick snapshots every document changed since its last snapshot.
func (a *Autosaver) Tick() {
	for _, s := range a.hub.All() {
		a.save(s)
	}
}

func (a *Autosaver) save(s *Session) {
	a.mu.Lock()
	version, has := a.saved[s]
	a.mu.Unlock()
	snap, ok, err := s.snapshot(version, has)
	if err != nil {
		log.Printf("autosave: %v", err)
		return
	}
	if !ok {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !snap.dirty {
		if has {
			a.Store.Remove(s.ID)
			delete(a.saved, s)
		}
		return
	}
	e := recovery.Entry{ID: s.ID, Title: snap.title, Path: snap.path, Saved: time.Now()}
	if err := a.Store.Save(e, snap.data); err != nil {
		log.Printf("autosave: %v", err)
		return
	}
	a.saved[s] = snap.version
}

// Forget removes a session's snapshot, e.g. when its window closes
// (the user already chose to save or discard).
func (a *Autosaver) Forget(s *Session) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Store.Remove(s.ID)
	delete(a.saved, s)
}

// Run ticks until stop is closed. interval is re-read every tick, so a
// preference change applies without restarting; 0 disables autosave.
func (a *Autosaver) Run(interval func() time.Duration, stop <-chan struct{}) {
	last := time.Now()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			d := interval()
			if d <= 0 || now.Sub(last) < d {
				continue
			}
			last = now
			a.Tick()
		}
	}
}
