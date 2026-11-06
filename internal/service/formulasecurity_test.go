package service

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"ww-excel/internal/document"
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/model"
)

func formulaState(t *testing.T, s *Session) DocState {
	t.Helper()
	result := make(chan DocState, 1)
	go func() { result <- s.State() }()
	select {
	case state := <-result:
		return state
	case <-time.After(time.Second):
		t.Fatal("formula failure left the document locked")
		return DocState{}
	}
}

// Do not let a lock regression hang test cleanup and hide the original failure.
func closeFormulaSession(s *Session) {
	if s.mu.TryLock() {
		defer s.mu.Unlock()
		s.doc.Close()
	}
}

func TestFormulaErrorsPreserveDocumentHistoryAndSaving(t *testing.T) {
	for _, tc := range []struct {
		name, formula string
		inject        bool
	}{
		{"REPT overflow", `=REPT(REPT("a",16384)&REPT("a",16384),562949953421312)`, false},
		{"unexpected evaluator panic", `=ABS(-1)`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.inject {
				// Keep the real function registry unchanged after this sequential test.
				def := eval.Lookup("ABS")
				original := def.Fn
				def.Fn = func(*eval.Args) eval.Value { panic("injected formula failure") }
				defer func() { def.Fn = original }()
			}
			sess, _ := newTestSession(false)
			defer closeFormulaSession(sess)
			sheets := NewSheetService(sess)
			docs := NewDocumentService(sess, &fakeDialogs{})
			dir := t.TempDir()
			if _, err := sheets.SetCell(bg, 0, 0, 0, "before"); err != nil {
				t.Fatal(err)
			}
			if err := sess.doc.SaveAs(filepath.Join(dir, "baseline.wwx")); err != nil {
				t.Fatal(err)
			}
			if _, err := sheets.SetCell(bg, 0, 0, 0, tc.formula); err != nil {
				t.Fatal(err)
			}
			state := formulaState(t, sess)
			if !state.Dirty || !state.CanUndo {
				t.Fatal("formula error was not recorded as an edit")
			}
			cell := sess.doc.Workbook().Sheets[0].Get(0, 0)
			if cell.Display() != model.ErrValue || cell.F == nil {
				t.Fatalf("formula result=%q formula=%v", cell.Display(), cell.F != nil)
			}
			if !sheets.Undo(bg) || sess.doc.Workbook().Sheets[0].Get(0, 0).Str != "before" || formulaState(t, sess).Dirty {
				t.Fatal("formula error broke undo or the save point")
			}
			if !sheets.Redo(bg) || sess.doc.Workbook().Sheets[0].Get(0, 0).Display() != model.ErrValue {
				t.Fatal("formula error broke redo")
			}
			if _, err := sheets.SetCell(bg, 0, 1, 0, "=1+2"); err != nil {
				t.Fatal(err)
			}
			if sess.doc.Workbook().Sheets[0].Get(1, 0).Num != 3 {
				t.Fatal("later editing stopped working")
			}
			if !docs.Save(bg) || formulaState(t, sess).Dirty {
				t.Fatal("saving after formula error failed")
			}
			for _, ext := range []string{".wwx", ".xlsx"} {
				path := filepath.Join(dir, "result"+ext)
				if err := sess.doc.SaveAs(path); err != nil {
					t.Fatal(err)
				}
				back, err := document.Load(path)
				if err != nil {
					t.Fatal(err)
				}
				sh := back.Workbook().Sheets[0]
				if sh.Get(0, 0).Display() != model.ErrValue || sh.Get(0, 0).F == nil || sh.Get(1, 0).Num != 3 {
					t.Fatalf("%s lost formula/error/healthy edit", ext)
				}
				back.Close()
			}
			if !sheets.Undo(bg) || !sheets.Undo(bg) || sess.doc.Workbook().Sheets[0].Get(0, 0).Str != "before" {
				t.Fatal("history became inconsistent after saving")
			}
			if !sheets.Redo(bg) || !sheets.Redo(bg) || formulaState(t, sess).Dirty {
				t.Fatal("redo did not return to the saved state")
			}
		})
	}
}

func TestSessionChangeReleasesLockOnUnexpectedPanic(t *testing.T) {
	sess, _ := newTestSession(false)
	defer closeFormulaSession(sess)
	before := sess.State()
	var failure any
	func() {
		defer func() { failure = recover() }()
		sess.change(false, func(*document.Document) (document.Change, bool) { panic("unexpected mutation failure") })
	}()
	if failure == nil {
		t.Fatal("unexpected mutation panic was silently swallowed")
	}
	if after := formulaState(t, sess); !reflect.DeepEqual(before, after) {
		t.Fatal("failure before mutation changed the document")
	}
	if _, err := NewSheetService(sess).SetCell(bg, 0, 0, 0, "usable"); err != nil {
		t.Fatal(err)
	}
}

func TestSessionChangeUnlocksBeforeSynchronousEvents(t *testing.T) {
	var sess *Session
	sess = NewSession(document.New(), func(string, any) { _ = sess.State() })
	defer closeFormulaSession(sess)
	done := make(chan error, 1)
	go func() { _, err := NewSheetService(sess).SetCell(bg, 0, 0, 0, "usable"); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("event callback queried a still-locked session")
	}
}
