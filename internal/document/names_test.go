package document

import (
	"path/filepath"
	"testing"
)

func TestDefinedNames(t *testing.T) {
	d := New()
	for r := 0; r < 3; r++ {
		enter(t, d, 0, r, 0, "10")
	}
	must(t, d)(d.SetNameCommand(nil, NameInput{Name: "銷售額", Scope: -1, RefersTo: "=工作表1!$A$1:$A$3"}))
	enter(t, d, 0, 5, 1, "=SUM(銷售額)")
	if display(d, 0, 5, 1) != "30" {
		t.Fatalf("SUM(name) = %q", display(d, 0, 5, 1))
	}
	enter(t, d, 0, 1, 0, "20")
	if display(d, 0, 5, 1) != "40" {
		t.Fatalf("recalc through name = %q", display(d, 0, 5, 1))
	}
	// Inserting a row inside the range grows the name.
	shift(t, d, 0, true, 1, 1)
	if n := d.FindName("銷售額", -1); n == nil || n.Formula != "工作表1!$A$1:$A$4" {
		t.Fatalf("name after insert = %+v", n)
	}
	// Renaming the sheet updates the name.
	cmd, _ := d.RenameSheetCommand(0, "資料")
	d.Exec(cmd)
	if n := d.FindName("銷售額", -1); n.Formula != "資料!$A$1:$A$4" {
		t.Fatalf("name after rename = %q", n.Formula)
	}
	d.Undo()
	if n := d.FindName("銷售額", -1); n.Formula != "工作表1!$A$1:$A$4" {
		t.Fatalf("undo rename = %q", n.Formula)
	}
	// Deleting the name breaks formulas; undo restores them.
	must(t, d)(d.DeleteNameCommand(d.FindName("銷售額", -1)))
	if display(d, 0, 6, 1) != "#NAME?" {
		t.Fatalf("deleted name = %q", display(d, 0, 6, 1))
	}
	d.Undo()
	if display(d, 0, 6, 1) != "40" {
		t.Fatalf("undo delete name = %q", display(d, 0, 6, 1))
	}
	// Invalid names are refused.
	for _, bad := range []string{"A1", "1abc", "a b", "R1C1", "XFD5"} {
		if _, err := d.SetNameCommand(nil, NameInput{Name: bad, Scope: -1, RefersTo: "=A1"}); err == nil {
			t.Errorf("name %q accepted", bad)
		}
	}
	if tgt, ok := d.Resolve(0, "銷售額"); !ok || tgt.Range.R1 != 3 {
		t.Fatalf("resolve name = %+v %v", tgt, ok)
	}
	if tgt, ok := d.Resolve(0, "C3:D4"); !ok || tgt.Range.C0 != 2 {
		t.Fatalf("resolve address = %+v", tgt)
	}

	// xlsx and wwx keep names.
	for _, ext := range []string{".xlsx", ".wwx"} {
		path := filepath.Join(t.TempDir(), "names"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if n := back.FindName("銷售額", -1); n == nil || n.Formula != "工作表1!$A$1:$A$4" || display(back, 0, 6, 1) != "40" {
			t.Fatalf("%s round trip: %+v %q", ext, n, display(back, 0, 6, 1))
		}
	}
}

func TestNotes(t *testing.T) {
	d := New()
	must(t, d)(d.NoteCommand(0, 2, 1, "記得核對", "andrew"))
	sh := d.Workbook().Sheets[0]
	if n := sh.Layout.Notes[[2]int{2, 1}]; n.Text != "記得核對" || n.Author != "andrew" {
		t.Fatalf("note = %+v", n)
	}
	shift(t, d, 0, true, 0, 2)
	if _, ok := sh.Layout.Notes[[2]int{4, 1}]; !ok {
		t.Fatalf("note did not move: %v", sh.Layout.Notes)
	}
	for _, ext := range []string{".xlsx", ".wwx"} {
		path := filepath.Join(t.TempDir(), "notes"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if n := back.Workbook().Sheets[0].Layout.Notes[[2]int{4, 1}]; n.Text != "記得核對" {
			t.Fatalf("%s note = %+v", ext, back.Workbook().Sheets[0].Layout.Notes)
		}
		// Edit and delete in place.
		if ext == ".xlsx" {
			must(t, back)(back.NoteCommand(0, 4, 1, "改過", "x"))
			must(t, back)(back.NoteCommand(0, 0, 0, "新的", "x"))
			if err := back.SaveAs(path); err != nil {
				t.Fatal(err)
			}
			again, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			notes := again.Workbook().Sheets[0].Layout.Notes
			if notes[[2]int{4, 1}].Text != "改過" || notes[[2]int{0, 0}].Text != "新的" || len(notes) != 2 {
				t.Fatalf("in-place notes = %+v", notes)
			}
		}
	}
	d.Undo()
	d.Undo()
	if len(sh.Layout.Notes) != 0 {
		t.Fatal("undo note")
	}
}
