package document

import (
	"path/filepath"
	"testing"
)

func TestTables(t *testing.T) {
	d := New()
	rows := [][]string{{"品項", "單價", "數量"}, {"蘋果", "30", "2"}, {"香蕉", "10", "5"}}
	for r, row := range rows {
		for c, v := range row {
			enter(t, d, 0, r, c, v)
		}
	}
	cmd, name, err := d.CreateTableCommand(0, rg(0, 0, 2, 2), true, "")
	must(t, d)(cmd, err)
	if name != "表格1" {
		t.Fatalf("name = %q", name)
	}
	if c := d.ExpandTableCommand(0, 1, 3); c == nil {
		t.Fatal("no column expansion")
	} else {
		d.Exec(c)
	}
	enter(t, d, 0, 1, 3, "=[@單價]*[@數量]")
	if editText(d, 0, 0, 3) != "欄4" {
		t.Fatalf("new column header = %q", editText(d, 0, 0, 3))
	}
	enter(t, d, 0, 6, 0, "=SUM(表格1[數量])")
	enter(t, d, 0, 7, 0, "=COUNTA(表格1[#All])")
	enter(t, d, 0, 8, 0, "=表格1[[#Headers],[單價]]")
	if display(d, 0, 1, 3) != "60" || display(d, 0, 6, 0) != "7" || display(d, 0, 7, 0) != "11" || display(d, 0, 8, 0) != "單價" {
		t.Fatalf("structured refs: %q %q %q %q", display(d, 0, 1, 3), display(d, 0, 6, 0), display(d, 0, 7, 0), display(d, 0, 8, 0))
	}
	// Typing below the table expands it; formulas follow.
	if c := d.ExpandTableCommand(0, 3, 2); c == nil {
		t.Fatal("no auto-expansion")
	} else {
		d.Exec(c)
	}
	enter(t, d, 0, 3, 2, "3")
	if display(d, 0, 6, 0) != "10" {
		t.Fatalf("SUM after expansion = %q", display(d, 0, 6, 0))
	}
	// Totals row.
	must(t, d)(d.ToggleTotalsCommand(0, "表格1"))
	if editText(d, 0, 4, 3) != "=SUBTOTAL(109,表格1[欄4])" || display(d, 0, 4, 3) != "60" || display(d, 0, 4, 0) != "合計" {
		t.Fatalf("totals = %q %q", editText(d, 0, 4, 3), display(d, 0, 4, 3))
	}
	// Rename updates formulas.
	must(t, d)(d.RenameTableCommand(0, "表格1", "水果"))
	if editText(d, 0, 6, 0) != "=SUM(水果[數量])" {
		t.Fatalf("after rename: %q", editText(d, 0, 6, 0))
	}
	d.Undo()
	if editText(d, 0, 6, 0) != "=SUM(表格1[數量])" {
		t.Fatalf("undo rename: %q", editText(d, 0, 6, 0))
	}
	// Round trips.
	for _, ext := range []string{".xlsx", ".wwx"} {
		path := filepath.Join(t.TempDir(), "table"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		tb, _ := back.Workbook().FindTable("表格1")
		if tb == nil || display(back, 0, 6, 0) != "10" || display(back, 0, 1, 3) != "60" {
			t.Fatalf("%s: table %+v, %q %q", ext, tb, display(back, 0, 6, 0), display(back, 0, 1, 3))
		}
	}
	// Converting to a range rewrites the references.
	must(t, d)(d.ConvertTableCommand(0, "表格1"))
	if editText(d, 0, 6, 0) != "=SUM($C$2:$C$4)" || editText(d, 0, 1, 3) != "=$B2*$C2" {
		t.Fatalf("after convert: %q %q", editText(d, 0, 6, 0), editText(d, 0, 1, 3))
	}
}
