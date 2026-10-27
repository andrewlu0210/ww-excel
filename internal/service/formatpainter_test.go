package service

import (
	"testing"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func TestFormatPainterSessionAndClipboard(t *testing.T) {
	session, rec := newTestSession(false)
	s := NewSheetService(session)
	st := session.doc.Workbook().StyleTable().Default()
	st.Font.Bold = true
	id := session.doc.Workbook().StyleTable().Intern(st)
	session.doc.Workbook().Sheets[0].Set(0, 0, model.Cell{S: id})
	clipboardWrites := 0
	s.clip.SetText = func(string) bool { clipboardWrites++; return true }
	if err := s.CaptureFormats(bg, 0, RangeDTO{}); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 0 || session.doc.Dirty() {
		t.Fatal("capture must not emit document changes")
	}
	if err := s.PaintFormats(bg, 0, []RangeDTO{{R0: 2, C0: 2, R1: 2, C1: 2}}); err != nil {
		t.Fatal(err)
	}
	if clipboardWrites != 0 || s.HasClip(bg) {
		t.Fatal("format painter must not touch the clipboard")
	}
	if rec.lastDiff(t).Select == nil {
		t.Fatal("painted target should be selected")
	}
	other, _ := newTestSession(false)
	if err := NewSheetService(other).PaintFormats(bg, 0, []RangeDTO{{}}); err == nil {
		t.Fatal("samples must be isolated between windows")
	}
	session.replace(document.New())
	if err := s.PaintFormats(bg, 0, []RangeDTO{{}}); err == nil {
		t.Fatal("replacing the document must clear the sample")
	}
}
