package service

import (
	"testing"
	"ww-excel/internal/model"
)

func TestFormatBorderPatchAndValidation(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	line := &BorderLineDTO{Style: "double", Color: "ff0000"}
	if err := svc.Format(bg, 0, []RangeDTO{{1, 1, 2, 2}}, FormatPatch{Borders: &BorderPatchDTO{Top: line, InsideV: line}}); err != nil {
		t.Fatal(err)
	}
	wb := sess.doc.Workbook()
	at := func(r, c int) model.Borders { return wb.Styles.Get(wb.Sheets[0].EffectiveStyle(r, c)).Border }
	if at(1, 1).Top.Color != "FF0000" || at(0, 1).Bottom != at(1, 1).Top || at(1, 1).Right != at(1, 2).Left {
		t.Fatal("service border conversion")
	}
	before := len(rec.diffs)
	for _, bad := range []*BorderLineDTO{{Style: "script"}, {Style: "thin", Color: "red"}, {Style: "thin", Color: "123456;"}} {
		if err := svc.Format(bg, 0, []RangeDTO{{1, 1, 2, 2}}, FormatPatch{Borders: &BorderPatchDTO{Left: bad}}); err == nil {
			t.Fatal("invalid patch accepted")
		}
	}
	if len(rec.diffs) != before {
		t.Fatal("invalid patch mutated document")
	}
	if err := svc.Format(bg, 0, []RangeDTO{{1, 1, 2, 2}}, FormatPatch{Borders: &BorderPatchDTO{Top: &BorderLineDTO{}}}); err != nil {
		t.Fatal(err)
	}
	if at(1, 1).Top.Style != "" || at(0, 1).Bottom.Style != "" || at(1, 1).Right.Style != "double" {
		t.Fatal("clear versus unchanged")
	}
}
func TestToolbarClearsBorderDialogSharedEdges(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	if err := svc.Format(bg, 0, []RangeDTO{{1, 1, 2, 2}}, FormatPatch{Borders: &BorderPatchDTO{Top: &BorderLineDTO{Style: "thick"}}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Format(bg, 0, []RangeDTO{{1, 1, 2, 2}}, FormatPatch{Border: "none"}); err != nil {
		t.Fatal(err)
	}
	wb := sess.doc.Workbook()
	if wb.Styles.Get(wb.Sheets[0].EffectiveStyle(0, 1)).Border.Bottom.Style != "" {
		t.Fatal("toolbar left mirrored old border")
	}
}
