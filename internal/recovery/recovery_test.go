package recovery

import (
	"testing"
	"time"

	"ww-excel/internal/model"
)

func TestSaveOrphansLoad(t *testing.T) {
	s := New(t.TempDir())
	wb := model.NewWorkbook()
	wb.Sheets[0].Set(2, 1, model.NumberCell(42))
	data, err := Encode(wb)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(Entry{ID: "a", Title: "預算.xlsx", Saved: time.Unix(100, 0)}, data); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(Entry{ID: "b", Title: "未命名", Saved: time.Unix(200, 0)}, data); err != nil {
		t.Fatal(err)
	}

	// Our own snapshots are not orphans while we run.
	if got := s.Orphans(); len(got) != 0 {
		t.Fatalf("own snapshots listed: %v", got)
	}

	// Seen from the next launch, after this process died.
	next := New(s.Dir)
	next.PID = s.PID + 1
	next.Alive = func(int) bool { return false }
	got := next.Orphans()
	if len(got) != 2 || got[0].ID != "b" || got[1].Title != "預算.xlsx" {
		t.Fatalf("orphans = %+v", got)
	}
	wb2, err := next.Load(got[1])
	if err != nil {
		t.Fatal(err)
	}
	if c := wb2.Sheets[0].Get(2, 1); c.Num != 42 {
		t.Fatalf("cell = %+v", c)
	}

	// A running owner keeps its snapshots.
	next.Alive = func(int) bool { return true }
	if got := next.Orphans(); len(got) != 0 {
		t.Fatal("snapshots of a running process listed")
	}

	next.Alive = func(int) bool { return false }
	next.Remove("a")
	next.Remove("missing")
	if got := next.Orphans(); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("after remove = %+v", got)
	}
}

func TestOrphansEmptyDir(t *testing.T) {
	s := New(t.TempDir() + "/none")
	if len(s.Orphans()) != 0 {
		t.Fatal("missing dir has orphans")
	}
}
