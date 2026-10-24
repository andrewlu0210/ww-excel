package service

import (
	"path/filepath"
	"testing"

	"ww-excel/internal/recovery"
)

func TestAutosave(t *testing.T) {
	sess, _ := newTestSession(false)
	hub := NewHub()
	hub.Add(1, sess)
	store := recovery.New(t.TempDir())
	a := NewAutosaver(store, hub)
	snapshots := func() int {
		m, _ := filepath.Glob(filepath.Join(store.Dir, "*.wwx"))
		return len(m)
	}

	a.Tick()
	if snapshots() != 0 {
		t.Fatal("clean document snapshotted")
	}

	svc := NewSheetService(sess)
	svc.SetCell(bg, 0, 0, 0, "1")
	a.Tick()
	if snapshots() != 1 {
		t.Fatal("dirty document not snapshotted")
	}
	v := a.saved[sess]
	a.Tick() // unchanged: no rewrite
	if a.saved[sess] != v {
		t.Fatal("unchanged document re-snapshotted")
	}

	// Undo back to the saved (empty) state removes the snapshot.
	svc.Undo(bg)
	a.Tick()
	if snapshots() != 0 {
		t.Fatal("snapshot kept for a clean document")
	}

	svc.SetCell(bg, 0, 0, 0, "2")
	a.Tick()
	a.Forget(sess)
	if snapshots() != 0 {
		t.Fatal("Forget kept the snapshot")
	}
}

func TestRecover(t *testing.T) {
	sess, _ := newTestSession(false)
	NewSheetService(sess).SetCell(bg, 0, 1, 1, "7")
	dir := t.TempDir()
	hub := NewHub()
	hub.Add(1, sess)
	crashed := recovery.New(dir)
	crashed.PID = -1
	NewAutosaver(crashed, hub).Tick()

	// The next launch: a pristine window gets the recovered workbook.
	next := recovery.New(dir)
	next.Alive = func(int) bool { return false }
	target, _ := newTestSession(false)
	ds := NewDocumentService(target, &fakeDialogs{})
	Recover(ds, bg, next)
	st := target.State()
	if !st.Dirty || st.Name != "未命名（已復原）" {
		t.Fatalf("state = %+v", st)
	}
	if v := NewSheetService(target).EditText(bg, 0, 1, 1); v != "7" {
		t.Fatalf("B2 = %q", v)
	}
	if len(next.Orphans()) != 0 {
		t.Fatal("recovered snapshot kept")
	}
}
