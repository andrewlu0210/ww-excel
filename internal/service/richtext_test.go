package service

import (
	"reflect"
	"testing"

	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func TestRichTextCommitDiffUndoRedo(t *testing.T) {
	red, black, bold := "FF0000", "000000", true
	wb := model.NewWorkbook()
	old := model.StringCell("標題內文")
	old.Rich = &model.RichText{Runs: []model.TextRun{
		{Text: "標題", Font: &model.RichFont{Color: &red, Bold: &bold}},
		{Text: "內文", Font: &model.RichFont{Color: &black}},
	}}
	wb.Sheets[0].Set(0, 0, old)
	d := document.FromWorkbook(wb, "")
	defer d.Close()
	rec := &recorder{}
	svc := NewSheetService(NewSession(d, rec.emit))
	// Both the cell editor and formula bar use this same commit endpoint.
	if _, err := svc.SetCell(bg, 0, 0, 0, "標題說明🙂"); err != nil {
		t.Fatal(err)
	}
	want := richTextDTO(&model.RichText{Runs: []model.TextRun{
		{Text: "標題", Font: old.Rich.Runs[0].Font},
		{Text: "說明🙂", Font: old.Rich.Runs[1].Font},
	}}, model.DefaultFont)
	assertDiff := func(text string, runs []TextRunDTO) {
		t.Helper()
		cells := rec.lastDiff(t).Cells
		if len(cells) != 1 || cells[0].V != text || !reflect.DeepEqual(cells[0].Runs, runs) {
			t.Fatalf("diff lost rich text: %+v, want %q %+v", cells, text, runs)
		}
		blocks := svc.GetBlocks(bg, 0, []BlockRef{{0, 0}}, 0).Blocks
		if !reflect.DeepEqual(blocks[0].Cells[0].Runs, runs) {
			t.Fatal("refreshed block lost rich text")
		}
	}
	assertDiff("標題說明🙂", want)
	if !svc.Undo(bg) {
		t.Fatal("cannot undo")
	}
	assertDiff(old.Str, richTextDTO(old.Rich, model.DefaultFont))
	if !svc.Redo(bg) {
		t.Fatal("cannot redo")
	}
	assertDiff("標題說明🙂", want)
	version := d.Version()
	if _, err := svc.SetCell(bg, 0, 0, 0, "標題說明🙂"); err != nil || d.Version() != version {
		t.Fatal("unchanged input should not create a change")
	}
}
