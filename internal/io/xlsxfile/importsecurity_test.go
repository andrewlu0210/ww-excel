package xlsxfile

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func importFixture(t *testing.T, parts map[string]string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "fixture.xlsx")
	f, e := os.Create(file)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for name, src := range parts {
		w, _ := z.Create(name)
		io.WriteString(w, src)
	}
	z.Close()
	f.Close()
	return file
}
func TestImportRejectsCustomPartsBeforeParsers(t *testing.T) {
	for _, test := range []struct{ name, part, src string }{
		{"custom deep XML", "customXml/item1.xml", strings.Repeat("<a>", 129) + strings.Repeat("</a>", 129)},
		{"malformed drawing", "xl/drawings/drawing1.xml", "<drawing>"},
		{"giant string", "xl/sharedStrings.xml", `<sst><si><t>` + strings.Repeat("x", 513<<10) + `</t></si></sst>`},
		{"formula nesting", "xl/worksheets/sheet1.xml", `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row><c><f>` + strings.Repeat("(", 129) + "1" + strings.Repeat(")", 129) + `</f></c></row></sheetData></worksheet>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			wb, src, err := Read(importFixture(t, map[string]string{test.part: test.src}))
			if err == nil || wb != nil || src != nil {
				if src != nil {
					src.Close()
				}
				t.Fatalf("accepted malicious part: %v", err)
			}
		})
	}
}
func TestImportCancellationLeavesNoSnapshot(t *testing.T) {
	file := importFixture(t, map[string]string{"a.xml": "<a/>"})
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wb, src, err := ReadContext(ctx, file)
	if wb != nil || src != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("snapshot leaked")
	}
}

// BenchmarkImportLarge compares the same parser with and without preflight.
// It is opt-in and never bypasses guards in application code.
func BenchmarkImportLarge(b *testing.B) {
	file := os.Getenv("WW_BIG_XLSX")
	if file == "" {
		b.Skip("set WW_BIG_XLSX")
	}
	for _, guarded := range []bool{false, true} {
		name := "ParserOnly"
		if guarded {
			name = "Guarded"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if guarded {
					wb, f, e := Read(file)
					if e != nil {
						b.Fatal(e)
					}
					if len(wb.Sheets) == 0 {
						b.Fatal("no sheets")
					}
					f.Close()
				} else {
					_, f, e := readXLSX(file, load)
					if e != nil {
						b.Fatal(e)
					}
					f.Close()
				}
			}
		})
	}
}
