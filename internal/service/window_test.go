package service

import (
	"testing"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func TestReplaceAppliesImportedWindowViewOnlyOnOpen(t *testing.T) {
	sess := NewSession(document.New(), func(string, any) {})
	defer func() { sess.doc.Close() }()
	calls := 0
	sess.ApplyWindowView = func(size model.WindowView) {
		calls++
		if size != (model.WindowView{Width: 1362, Height: 883}) {
			t.Fatal(size)
		}
		// The callback runs outside the session lock and sees the loaded document.
		if sess.State().Dirty {
			t.Fatal("opening changed the document")
		}
	}
	wb := model.NewWorkbook()
	wb.OpenWindowView = model.WindowView{Width: 1362, Height: 883}
	sess.replace(document.FromWorkbook(wb, "test.xlsx"))
	if calls != 1 {
		t.Fatal("opening did not apply the size")
	}
	sess.PublishState()
	if calls != 1 {
		t.Fatal("state update reapplied the size")
	}
	sess.replace(document.New())
	if calls != 1 {
		t.Fatal("missing size resized the window")
	}
}

func TestReplaceAppliesPositionWithoutSize(t *testing.T) {
	sess := NewSession(document.New(), func(string, any) {})
	defer func() { sess.doc.Close() }()
	calls := 0
	sess.ApplyWindowView = func(view model.WindowView) {
		calls++
		if view.Position == nil || *view.Position != (model.WindowPosition{X: 72, Y: 33}) {
			t.Fatal(view)
		}
		if sess.State().Dirty {
			t.Fatal("opening changed the document")
		}
	}
	wb := model.NewWorkbook()
	wb.OpenWindowView.Position = &model.WindowPosition{X: 72, Y: 33}
	sess.replace(document.FromWorkbook(wb, "position.xlsx"))
	sess.PublishState()
	if calls != 1 {
		t.Fatalf("callback count %d", calls)
	}
}
