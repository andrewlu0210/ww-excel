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

func TestRichSelectionCommitAtomicAndValidation(t *testing.T) {
	wb := model.NewWorkbook()
	wb.Sheets[0].Set(0, 0, model.StringCell("標題內文"))
	d := document.FromWorkbook(wb, "")
	defer d.Close()
	svc := NewSheetService(NewSession(d, func(string, any) {}))
	font := fontDTO(model.DefaultFont)
	red := font
	red.Color = "FF0000"
	red.Bold = true
	runs := []TextRunDTO{{Text: "標題", Font: red}, {Text: "內文🙂", Font: font}}
	if _, err := svc.SetRichCell(bg, 0, 0, 0, "標題內文🙂", runs, 37); err != nil {
		t.Fatal(err)
	}
	got := wb.Sheets[0].Get(0, 0)
	if got.Str != "標題內文🙂" || !reflect.DeepEqual(richTextDTO(got.Rich, model.DefaultFont), runs) {
		t.Fatalf("lost runs: %+v", got)
	}
	if wb.Sheets[0].Layout.RowHeight(0) != 37 {
		t.Fatal("rich font row did not grow")
	}
	version := d.Version()
	if _, err := svc.SetRichCell(bg, 0, 0, 0, "標題內文🙂", runs, 0); err != nil || d.Version() != version {
		t.Fatal("unchanged rich commit creates history")
	}
	if _, err := svc.SetRichCell(bg, 0, 0, 0, "wrong", runs, 0); err == nil || d.Version() != version {
		t.Fatal("mismatched runs should be rejected atomically")
	}
	if !svc.Undo(bg) || wb.Sheets[0].Get(0, 0).Rich != nil || wb.Sheets[0].Get(0, 0).Str != "標題內文" {
		t.Fatal("undo did not restore original cell")
	}
	if wb.Sheets[0].Layout.RowHeight(0) != model.DefaultRowHeight {
		t.Fatal("undo lost original row height")
	}
	if !svc.Redo(bg) || wb.Sheets[0].Get(0, 0).Rich == nil {
		t.Fatal("redo did not restore rich cell")
	}
	if wb.Sheets[0].Layout.RowHeight(0) != 37 {
		t.Fatal("redo lost row growth")
	}
	for _, text := range []string{"'123", "'=SUM(A1)", "''文字"} {
		input := []TextRunDTO{{Text: text[:1], Font: font}, {Text: text[1:], Font: red}}
		if _, err := svc.SetRichCell(bg, 0, 1, 0, text, input, 0); err != nil {
			t.Fatal(err)
		}
		cell := wb.Sheets[0].Get(1, 0)
		if cell.Str != text[1:] || cell.F != nil || cell.Rich == nil || cell.Rich.Runs[0].Text != text[1:] {
			t.Fatalf("literal text: %+v", cell)
		}
	}
	if _, err := svc.SetRichCell(bg, 0, 2, 0, "=1+1", []TextRunDTO{{Text: "=1+1", Font: red}}, 0); err == nil {
		t.Fatal("rich formula should not be accepted")
	}
}

func TestRichTextConversionUsesOrdinaryInput(t *testing.T) {
	d := document.New()
	defer d.Close()
	svc := NewSheetService(NewSession(d, func(string, any) {}))
	for _, text := range []string{"123", "2026/11/3", ""} {
		runs := []TextRunDTO{{Text: text, Font: fontDTO(model.DefaultFont)}}
		if _, err := svc.SetRichCell(bg, 0, 0, 0, text, runs, 40); err != nil {
			t.Fatal(err)
		}
		cell := d.Workbook().Sheets[0].Get(0, 0)
		if cell.Rich != nil || d.Workbook().Sheets[0].Layout.RowHeight(0) != 20 {
			t.Fatal("conversion retained stale rich text or changed row height")
		}
		if text != "" && cell.Kind != model.KindNumber {
			t.Fatal("conversion lost numeric/date input")
		}
	}
}

func TestRichCommitHeightPreservesTableGrowthAndManualRows(t *testing.T) {
	d := document.New()
	defer d.Close()
	svc := NewSheetService(NewSession(d, func(string, any) {}))
	sh := d.Workbook().Sheets[0]
	sh.Layout.Tables = []model.Table{{Name: "Sales", Range: model.Range{R0: 0, C0: 0, R1: 1, C1: 1}}}
	runs := []TextRunDTO{{Text: "標題", Font: fontDTO(model.DefaultFont)}}
	if _, err := svc.SetRichCell(bg, 0, 2, 0, "標題", runs, 40); err != nil {
		t.Fatal(err)
	}
	if sh.Layout.Tables[0].Range.R1 != 2 || sh.Layout.RowHeight(2) != 40 {
		t.Fatal("row growth replaced expanded table")
	}
	svc.Undo(bg)
	if sh.Layout.Tables[0].Range.R1 != 1 || sh.Layout.RowHeight(2) != 20 || sh.Get(2, 0).HasContent() {
		t.Fatal("undo did not restore table/height/value")
	}
	svc.Redo(bg)
	if sh.Layout.Tables[0].Range.R1 != 2 || sh.Layout.RowHeight(2) != 40 {
		t.Fatal("redo did not restore table/height")
	}
	sh.Layout.Rows[4] = model.Size{Px: 25}
	if _, err := svc.SetRichCell(bg, 0, 4, 0, "標題", runs, 40); err != nil {
		t.Fatal(err)
	}
	if sh.Layout.RowHeight(4) != 25 {
		t.Fatal("manual row height changed")
	}
	v := ValidationDTO{Type: "textLength", Operator: "lessThanOrEqual", Formula1: "1", ErrorStyle: "stop"}
	if err := svc.SetValidation(bg, 0, []RangeDTO{{R0: 5, C0: 0, R1: 5, C1: 0}}, &v); err != nil {
		t.Fatal(err)
	}
	version := d.Version()
	if _, err := svc.SetRichCell(bg, 0, 5, 0, "標題", runs, 40); err == nil || d.Version() != version || sh.Get(5, 0).HasContent() || sh.Layout.RowHeight(5) != 20 {
		t.Fatal("invalid rich input was not rejected atomically")
	}
}
