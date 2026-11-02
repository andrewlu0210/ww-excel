package document

import (
	"path/filepath"
	"testing"

	"ww-excel/internal/command"
	"ww-excel/internal/model"
)

func TestEditedRichTextRoundTrips(t *testing.T) {
	red, black, name, size := "FF0000", "000000", "Arial", 16.0
	yes, no := true, false
	d := New()
	defer d.Close()
	old := model.StringCell("標題內文")
	old.Rich = &model.RichText{Runs: []model.TextRun{
		{Text: "標題", Font: &model.RichFont{Name: &name, Size: &size, Color: &red, Bold: &yes, Italic: &yes, Underline: &yes, Strike: &yes}},
		{Text: "內文", Font: &model.RichFont{Color: &black, Bold: &no, Italic: &no, Underline: &no, Strike: &no}},
	}}
	d.Workbook().Sheets[0].Set(0, 0, old)
	path := filepath.Join(t.TempDir(), "source.xlsx")
	if err := d.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	in, err := loaded.InputCell(0, "標題說明🙂\n第二行")
	if err != nil {
		t.Fatal(err)
	}
	loaded.Exec(command.SetCell(loaded.Workbook(), 0, 0, 0, in.Cell, in.NumFmt, in.Wrap))
	assertCell := func(d *Document) {
		t.Helper()
		sh := d.Workbook().Sheets[0]
		got := sh.Get(0, 0)
		if got.Str != in.Cell.Str || got.Rich == nil || len(got.Rich.Runs) != 2 {
			t.Fatalf("text/runs lost: %+v, rich=%+v", got, got.Rich)
		}
		base := d.Workbook().StyleTable().Get(sh.EffectiveStyle(0, 0))
		if !base.Align.Wrap {
			t.Fatal("multiline edit lost wrapping")
		}
		for i, run := range got.Rich.Runs {
			wantText := []string{"標題", "說明🙂\n第二行"}[i]
			if run.Text != wantText || run.Font.Resolve(base.Font) != old.Rich.Runs[i].Font.Resolve(base.Font) {
				t.Fatalf("run %d lost text/font: %+v", i, run)
			}
		}
	}
	assertCell(loaded)
	// Exercise saving over the imported XLSX, a second save, and native conversion.
	for _, ext := range []string{".xlsx", ".xlsx", ".wwx", ".xlsx"} {
		if ext != ".xlsx" || filepath.Ext(path) != ".xlsx" {
			path = filepath.Join(t.TempDir(), "edited"+ext)
		}
		if err := loaded.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		defer back.Close()
		assertCell(back)
		loaded = back
	}
}
