package document

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"ww-excel/internal/command"
	"ww-excel/internal/model"
)

func enter(t *testing.T, d *Document, sheet, row, col int, text string) {
	t.Helper()
	in, err := d.InputCell(sheet, text)
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(command.SetCell(d.Workbook(), sheet, row, col, in.Cell, in.NumFmt, in.Wrap))
}

func display(d *Document, sheet, row, col int) string {
	return d.Workbook().Sheets[sheet].Get(row, col).Display()
}

func numFmt(d *Document, sheet, row, col int) string {
	sh := d.Workbook().Sheets[sheet]
	return d.Workbook().StyleTable().Get(sh.EffectiveStyle(row, col)).NumFmt
}

func TestInputDetection(t *testing.T) {
	d := New()
	d.Now = func() time.Time { return time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC) }
	enter(t, d, 0, 0, 0, "2026/10/23")
	enter(t, d, 0, 1, 0, "12.5%")
	enter(t, d, 0, 2, 0, "=A1+7")
	enter(t, d, 0, 3, 0, "=TODAY()")
	enter(t, d, 0, 4, 0, "'2026/10/23")
	enter(t, d, 0, 5, 0, "第一行\n第二行")

	if got := d.Workbook().Sheets[0].Get(0, 0); got.Kind != model.KindNumber || got.Num != 46318 || numFmt(d, 0, 0, 0) != "yyyy/m/d" {
		t.Fatalf("date input = %+v %q", got, numFmt(d, 0, 0, 0))
	}
	if display(d, 0, 1, 0) != "0.125" || numFmt(d, 0, 1, 0) != "0.00%" {
		t.Fatalf("percent = %q %q", display(d, 0, 1, 0), numFmt(d, 0, 1, 0))
	}
	if numFmt(d, 0, 2, 0) != "yyyy/m/d" {
		t.Fatalf("=A1+7 with a date in A1 is a date; got %q", numFmt(d, 0, 2, 0))
	}
	if numFmt(d, 0, 3, 0) != "yyyy/m/d" {
		t.Fatal("TODAY() gets a date format")
	}
	if got := d.Workbook().Sheets[0].Get(4, 0); got.Kind != model.KindString {
		t.Fatal("apostrophe keeps text")
	}
	if !d.Workbook().StyleTable().Get(d.Workbook().Sheets[0].EffectiveStyle(5, 0)).Align.Wrap {
		t.Fatal("line break turns on wrap")
	}
}

func TestSheetOperations(t *testing.T) {
	d := New()
	enter(t, d, 0, 0, 0, "5")
	cmd, err := d.AddSheetCommand(1, d.NextSheetName())
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	if d.Workbook().Sheets[1].Name != "工作表2" {
		t.Fatalf("new sheet = %q", d.Workbook().Sheets[1].Name)
	}
	enter(t, d, 1, 0, 0, "=工作表1!A1*2")
	if display(d, 1, 0, 0) != "10" {
		t.Fatalf("cross-sheet = %q", display(d, 1, 0, 0))
	}

	// Rename updates references.
	cmd, _ = d.RenameSheetCommand(0, "銷售 資料")
	d.Exec(cmd)
	if got := d.Workbook().Sheets[1].Get(0, 0).EditText(); got != "='銷售 資料'!A1*2" {
		t.Fatalf("renamed ref = %q", got)
	}
	enter(t, d, 0, 0, 0, "6")
	if display(d, 1, 0, 0) != "12" {
		t.Fatal("dependency survives rename")
	}

	// Validation.
	if _, err := d.RenameSheetCommand(1, "銷售 資料"); err == nil {
		t.Fatal("duplicate name accepted")
	}
	if _, err := d.RenameSheetCommand(1, "a/b"); err == nil {
		t.Fatal("invalid character accepted")
	}

	// Move, then delete: references become #REF!; undo restores.
	cmd, _ = d.MoveSheetCommand(1, 0)
	d.Exec(cmd)
	if d.Workbook().Sheets[0].Name != "工作表2" || display(d, 0, 0, 0) != "12" {
		t.Fatal("move keeps formulas working")
	}
	cmd, _ = d.DeleteSheetCommand(1)
	d.Exec(cmd)
	if got := d.Workbook().Sheets[0].Get(0, 0).EditText(); got != "=#REF!*2" || display(d, 0, 0, 0) != "#REF!" {
		t.Fatalf("after delete: %q = %q", got, display(d, 0, 0, 0))
	}
	d.Undo()
	if got := d.Workbook().Sheets[0].Get(0, 0).EditText(); got != "='銷售 資料'!A1*2" || display(d, 0, 0, 0) != "12" {
		t.Fatalf("undo delete: %q = %q", got, display(d, 0, 0, 0))
	}
	if _, err := New().DeleteSheetCommand(0); err == nil {
		t.Fatal("deleting the last sheet must fail")
	}
}

func TestXLSXInPlaceThroughDocument(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "report.xlsx")
	x := excelize.NewFile()
	x.SetCellValue("Sheet1", "A1", 3)
	x.SetCellFormula("Sheet1", "B1", "A1*2")
	x.AddComment("Sheet1", excelize.Comment{Cell: "A1", Author: "QA", Text: "保留"})
	x.SaveAs(src)
	x.Close()

	d, err := Load(src)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if display(d, 0, 0, 1) != "6" {
		t.Fatalf("formula computed on load = %q", display(d, 0, 0, 1))
	}
	enter(t, d, 0, 0, 0, "10")
	cmd, _ := command.ApplyStyle(d.Workbook(), 0, []model.Range{{R0: 0, C0: 1, R1: 0, C1: 1}}, "粗體",
		func(s model.Style, _ command.At) model.Style { s.Font.Bold = true; return s })
	d.Exec(cmd)
	d.Exec(command.SetSizes(d.Workbook(), 0, false, []int{1}, 120))
	if err := d.SaveAs(src); err != nil {
		t.Fatal(err)
	}
	if d.Dirty() {
		t.Fatal("saved document is dirty")
	}

	y, err := excelize.OpenFile(src)
	if err != nil {
		t.Fatal(err)
	}
	defer y.Close()
	if v, _ := y.GetCellValue("Sheet1", "A1"); v != "10" {
		t.Fatalf("A1 = %q", v)
	}
	if f, _ := y.GetCellFormula("Sheet1", "B1"); f != "A1*2" {
		t.Fatalf("B1 formula = %q", f)
	}
	if c, _ := y.GetComments("Sheet1"); len(c) != 1 {
		t.Fatal("comment lost")
	}
	id, _ := y.GetCellStyle("Sheet1", "B1")
	if st, _ := y.GetStyle(id); st.Font == nil || !st.Font.Bold {
		t.Fatal("bold not saved")
	}
	if w, _ := y.GetColWidth("Sheet1", "B"); w < 17 || w > 18 {
		t.Fatalf("column width = %v", w)
	}

	// A second save after more edits still works on the same source.
	enter(t, d, 0, 2, 0, "second")
	if err := d.SaveAs(src); err != nil {
		t.Fatal(err)
	}
	d2, err := Load(src)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	if display(d2, 0, 2, 0) != "second" || display(d2, 0, 0, 1) != "20" {
		t.Fatalf("reload: A3 = %q, B1 = %q", display(d2, 0, 2, 0), display(d2, 0, 0, 1))
	}
}
