package document

import (
	"path/filepath"
	"testing"

	"ww-excel/internal/model"
)

func paint(t *testing.T, d *Document, sample *FormatSample, sheet int, area model.Range) {
	t.Helper()
	cmd, err := d.PaintFormats(sample, sheet, []model.Range{area})
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
}

func TestFormatPainterPatternAndUndo(t *testing.T) {
	d := New()
	defer d.Close()
	sh, styles := d.wb.Sheets[0], d.wb.StyleTable()
	bold := styles.Default()
	bold.Font.Bold = true
	bold.Fill = "FFCC00"
	bold.NumFmt = "0.00%"
	italic := styles.Default()
	italic.Font.Italic = true
	italic.Align.Wrap = true
	a, b := styles.Intern(bold), styles.Intern(italic)
	sh.Set(0, 0, model.Cell{S: a})
	sh.Layout.RowStyles[1] = b
	sample, err := d.CaptureFormats(0, model.Range{R1: 1, C1: 1})
	if err != nil {
		t.Fatal(err)
	}
	if d.Dirty() || d.History().CanUndo() {
		t.Fatal("capturing formats must not create an edit")
	}
	// Later source edits do not alter the sample. Existing merges stay intact.
	sh.Set(0, 0, model.Cell{S: b})
	sh.Layout.Merges = []model.Range{{R0: 8, C0: 5, R1: 8, C1: 6}}
	in, err := d.InputCell(0, "=1+2")
	if err != nil {
		t.Fatal(err)
	}
	sh.Set(5, 5, in.Cell)
	formula := sh.Get(5, 5).F
	paint(t, d, sample, 0, model.Range{R0: 5, C0: 5, R1: 7, C1: 7})
	if got := sh.Get(5, 5); got.F != formula || styles.Get(got.S) != bold {
		t.Fatalf("formula/style changed incorrectly: %+v", got)
	}
	if styles.Get(sh.EffectiveStyle(6, 6)) != italic || styles.Get(sh.EffectiveStyle(7, 7)) != bold {
		t.Fatal("source pattern was not tiled, including inherited styles")
	}
	if len(sh.Layout.Merges) != 1 {
		t.Fatal("painting must not alter merges")
	}
	if d.History().UndoLabel() != "複製格式" {
		t.Fatal("incorrect undo label")
	}
	d.Undo()
	if sh.Get(5, 5).S != 0 || sh.Get(5, 5).F != formula {
		t.Fatal("undo did not restore the target")
	}
	d.Redo()
	if styles.Get(sh.EffectiveStyle(5, 5)) != bold {
		t.Fatal("redo did not restore painted styles")
	}
	paint(t, d, sample, 0, model.Range{R0: 10, C0: 10, R1: 10, C1: 10})
	if styles.Get(sh.EffectiveStyle(11, 11)) != italic {
		t.Fatal("single-cell target must expand to source dimensions")
	}
}

func TestFormatPainterDefaultStyleRoundTrip(t *testing.T) {
	for _, ext := range []string{".wwx", ".xlsx"} {
		t.Run(ext, func(t *testing.T) {
			d := New()
			defer d.Close()
			sh, table := d.wb.Sheets[0], d.wb.StyleTable()
			colored := table.Default()
			colored.Fill = "FF0000"
			sh.Layout.ColStyles[4] = table.Intern(colored)
			sh.Set(4, 4, model.StringCell("keep"))
			sample, err := d.CaptureFormats(0, model.Range{})
			if err != nil {
				t.Fatal(err)
			}
			paint(t, d, sample, 0, model.Range{R0: 4, C0: 4, R1: 4, C1: 4})
			if sh.EffectiveStyle(4, 4) == 0 || table.Get(sh.EffectiveStyle(4, 4)).Fill != "" {
				t.Fatal("default format did not override column formatting")
			}
			path := filepath.Join(t.TempDir(), "paint"+ext)
			if err := d.SaveAs(path); err != nil {
				t.Fatal(err)
			}
			back, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			defer back.Close()
			cell := back.wb.Sheets[0].Get(4, 4)
			if cell.Str != "keep" || back.wb.StyleTable().Get(back.wb.Sheets[0].EffectiveStyle(4, 4)).Fill != "" {
				t.Fatalf("round trip lost explicit default: %+v", cell)
			}
		})
	}
}

func TestFormatPainterCrossSheetAndWholeColumn(t *testing.T) {
	d := New()
	defer d.Close()
	d.wb.Sheets = append(d.wb.Sheets, model.NewSheet("Target"))
	st := d.wb.StyleTable().Default()
	st.Font.Bold = true
	id := d.wb.StyleTable().Intern(st)
	d.wb.Sheets[0].Set(0, 0, model.Cell{S: id})
	sample, err := d.CaptureFormats(0, model.Range{})
	if err != nil {
		t.Fatal(err)
	}
	paint(t, d, sample, 1, model.Range{C0: 3, C1: 3, R1: model.MaxRows - 1})
	sh := d.wb.Sheets[1]
	if sh.EffectiveStyle(model.MaxRows-1, 3) != id {
		t.Fatal("whole-column formatting failed")
	}
	count := 0
	sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(int, int, model.Cell) { count++ })
	if count != 0 {
		t.Fatal("whole-column painting should not materialize empty cells")
	}
	other := New()
	defer other.Close()
	if _, err := other.PaintFormats(sample, 0, []model.Range{{}}); err == nil {
		t.Fatal("sample must not be used with another workbook")
	}
	if _, err := d.CaptureFormats(0, model.Range{R1: model.MaxRows - 1, C1: 1}); err == nil {
		t.Fatal("oversized capture must fail")
	}
}
