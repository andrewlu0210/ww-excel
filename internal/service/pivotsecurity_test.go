package service

import (
	"reflect"
	"testing"

	"ww-excel/internal/document"
	"ww-excel/internal/testutil"
)

func TestInvalidPivotOpenPreservesExistingDocuments(t *testing.T) {
	for _, mutation := range []string{"negative-data-index", "out-of-range-data-index", "empty-cache-fields", "short-cache-fields"} {
		t.Run(mutation, func(t *testing.T) {
			sess, _ := newTestSession(false)
			defer sess.Close()
			sheets := NewSheetService(sess)
			if _, err := sheets.SetCell(bg, 0, 0, 0, "unsaved"); err != nil {
				t.Fatal(err)
			}
			other, _ := newTestSession(false)
			defer other.Close()
			if _, err := NewSheetService(other).SetCell(bg, 0, 0, 0, "other document"); err != nil {
				t.Fatal(err)
			}
			before, otherBefore := sess.State(), other.State()
			dlg := &fakeDialogs{}
			docs := NewDocumentService(sess, dlg)
			windows, recent := 0, 0
			docs.OpenWindow = func(d *document.Document) { windows++; d.Close() }
			docs.Opened = func(string) { recent++ }
			if docs.OpenFile(bg, testutil.PivotXLSX(t, mutation)) {
				t.Fatal("malformed pivot opened")
			}
			if windows != 0 || recent != 0 || len(dlg.errors) != 1 {
				t.Fatalf("failed open side effects: windows=%d recent=%d errors=%v", windows, recent, dlg.errors)
			}
			if !reflect.DeepEqual(before, sess.State()) || !reflect.DeepEqual(otherBefore, other.State()) {
				t.Fatal("failed open changed an existing document")
			}
			if !docs.OpenFile(bg, testutil.PivotXLSX(t, "valid")) || windows != 1 || recent != 1 {
				t.Fatal("valid workbook cannot be opened after a failed import")
			}
			if _, err := sheets.SetCell(bg, 0, 1, 0, "still editable"); err != nil {
				t.Fatal(err)
			}
			sheets.Undo(bg)
			if got := sess.doc.Workbook().Sheets[0].Get(0, 0).Str; got != "unsaved" {
				t.Fatal("unsaved content lost")
			}
			if !sess.doc.Workbook().Sheets[0].Get(1, 0).IsEmpty() {
				t.Fatal("undo stopped working")
			}
		})
	}
}
