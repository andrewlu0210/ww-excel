package document

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func shift(t *testing.T, d *Document, sheet int, rows bool, at, count int) {
	t.Helper()
	cmd, err := d.ShiftCommand(sheet, rows, at, count)
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
}

func editText(d *Document, sheet, row, col int) string {
	return d.Workbook().Sheets[sheet].Get(row, col).EditText()
}

func TestInsertDeleteAdjustsFormulas(t *testing.T) {
	d := New()
	cmd, _ := d.AddSheetCommand(1, "其他")
	d.Exec(cmd)
	for r := 0; r < 5; r++ {
		enter(t, d, 0, r, 0, "1")
	}
	enter(t, d, 0, 6, 0, "=SUM(A1:A5)")
	enter(t, d, 0, 7, 0, "=A3*2")
	enter(t, d, 1, 0, 0, "=工作表1!A5+工作表1!$A$1")

	shift(t, d, 0, true, 2, 2) // insert two rows above row 3
	if got := editText(d, 0, 8, 0); got != "=SUM(A1:A7)" {
		t.Fatalf("range should grow: %q", got)
	}
	if got := editText(d, 0, 9, 0); got != "=A5*2" {
		t.Fatalf("reference should move: %q", got)
	}
	if got := editText(d, 1, 0, 0); got != "=工作表1!A7+工作表1!$A$1" {
		t.Fatalf("other sheet: %q", got)
	}
	enter(t, d, 0, 2, 0, "10")
	if display(d, 0, 8, 0) != "15" {
		t.Fatalf("SUM after insert = %q", display(d, 0, 8, 0))
	}

	shift(t, d, 0, true, 4, -1) // delete the row holding A5 (old A3)
	if got := editText(d, 0, 8, 0); got != "=#REF!*2" {
		t.Fatalf("deleted reference: %q", got)
	}
	d.Undo()
	if got := editText(d, 0, 9, 0); got != "=A5*2" || display(d, 0, 9, 0) != "2" {
		t.Fatalf("undo delete: %q = %q", got, display(d, 0, 9, 0))
	}
	d.Undo() // the "10"
	d.Undo() // the insertion
	if got := editText(d, 0, 6, 0); got != "=SUM(A1:A5)" || display(d, 0, 6, 0) != "5" {
		t.Fatalf("undo insert: %q = %q", got, display(d, 0, 6, 0))
	}
	if got := editText(d, 1, 0, 0); got != "=工作表1!A5+工作表1!$A$1" {
		t.Fatalf("undo restores other sheets: %q", got)
	}

	shift(t, d, 0, false, 0, 1) // insert a column before A
	if got := editText(d, 0, 6, 1); got != "=SUM(B1:B5)" {
		t.Fatalf("column insert: %q", got)
	}
}

func TestShiftReplaysOnXLSX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.xlsx")
	x := excelize.NewFile()
	x.SetCellValue("Sheet1", "A1", "top")
	x.SetCellValue("Sheet1", "A3", "moves")
	x.AddComment("Sheet1", excelize.Comment{Cell: "A3", Author: "QA", Text: "跟著移動"})
	x.SaveAs(path)
	x.Close()

	d, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	shift(t, d, 0, true, 1, 2) // insert 2 rows above row 2
	enter(t, d, 0, 1, 0, "new")
	if err := d.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	y, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer y.Close()
	if v, _ := y.GetCellValue("Sheet1", "A5"); v != "moves" {
		t.Fatalf("A5 = %q", v)
	}
	if v, _ := y.GetCellValue("Sheet1", "A2"); v != "new" {
		t.Fatalf("A2 = %q", v)
	}
	comments, _ := y.GetComments("Sheet1")
	if len(comments) != 1 || comments[0].Cell != "A5" {
		t.Fatalf("comment should move with its cell: %+v", comments)
	}
}
