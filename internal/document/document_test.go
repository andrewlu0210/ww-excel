package document

import (
	"os"
	"path/filepath"
	"testing"

	"ww-excel/internal/command"
	"ww-excel/internal/model"
)

func TestSaveLoadAndDirty(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"book.wwx", "data.csv"} {
		t.Run(name, func(t *testing.T) {
			d := New()
			if d.Name() != UntitledName || d.Dirty() {
				t.Fatal("new document should be untitled and clean")
			}
			v0 := d.Version()
			d.Exec(command.EnterText(d.Workbook(), 0, 1, 2, "台北"))
			if !d.Dirty() || d.Version() == v0 {
				t.Fatal("edit should dirty and bump version")
			}

			path := filepath.Join(dir, name)
			if err := d.SaveAs(path); err != nil {
				t.Fatal(err)
			}
			if d.Dirty() || d.Name() != name {
				t.Fatalf("after save: dirty=%v name=%q", d.Dirty(), d.Name())
			}
			if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o644 {
				t.Fatalf("mode = %v, want 0644", fi.Mode().Perm())
			}

			loaded, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := loaded.Workbook().Sheets[0].Get(1, 2); got != model.StringCell("台北") {
				t.Fatalf("loaded cell = %+v", got)
			}
			if loaded.Dirty() {
				t.Fatal("loaded document should be clean")
			}

			// No temp files left behind.
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if filepath.Ext(e.Name()) == ".tmp" {
					t.Fatalf("leftover temp file %s", e.Name())
				}
			}
		})
	}
}

func TestSaveFailureKeepsState(t *testing.T) {
	d := New()
	d.Exec(command.EnterText(d.Workbook(), 0, 0, 0, "1"))
	if _, ok := d.Exec(command.EnterText(d.Workbook(), 0, 0, 0, "1")); ok {
		t.Fatal("no-op edit should not execute")
	}
	if err := d.SaveAs(filepath.Join(t.TempDir(), "missing-dir", "x.wwx")); err == nil {
		t.Fatal("expected error")
	}
	if !d.Dirty() || d.Path() != "" {
		t.Fatal("failed save must not mark saved or change path")
	}
}

func TestFormatForPath(t *testing.T) {
	if FormatForPath("/a/B.CSV") != FormatCSV || FormatForPath("x.wwx") != FormatNative || FormatForPath("x") != FormatNative {
		t.Fatal("format detection")
	}
}
