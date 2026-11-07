package xlsxfile

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"ww-excel/internal/model"
)

func TestSparseAxisMetadataPreservesLayout(t *testing.T) {
	raw := []byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cols><col min="1" max="6" width="22" hidden="1" customWidth="1"/></cols><sheetData><row r="1"/><row r="3" ht="30" customHeight="1" s="7" customFormat="1"/><row r="5"><c r="B5"><v>42</v></c></row></sheetData><extLst><ext uri="keep"/></extLst></worksheet>`)
	got, err := patchAxisStyles(raw, axisStylePlan{columns: map[int]int{2: 8, 3: 8}, rows: map[int]*int{2: nil, 3: axisInt(9), 999999: axisInt(10)}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{`min="3" max="4"`, `width="22" hidden="1" customWidth="1" style="8"`, `<row r="3" ht="30" customHeight="1"/>`, `<row r="4" s="9" customFormat="1"/>`, `<row r="1000000" s="10" customFormat="1"/>`, `<c r="B5"><v>42</v></c>`, `<ext uri="keep"/>`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	if strings.Contains(s, `<row r="1"`) || strings.Contains(s, `s="7"`) {
		t.Fatal(s)
	}
	d := xml.NewDecoder(bytes.NewReader(got))
	for {
		_, err = d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}
func TestSparseAxisEmptySheet(t *testing.T) {
	for _, body := range []string{`<sheetData/>`, `<sheetData></sheetData>`} {
		got, err := patchAxisStyles([]byte(`<worksheet>`+body+`</worksheet>`), axisStylePlan{columns: map[int]int{1: 4}, rows: map[int]*int{8: axisInt(3)}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), `<cols><col min="2" max="2" style="4"/></cols><sheetData><row r="9" s="3" customFormat="1"/></sheetData>`) {
			t.Fatal(string(got))
		}
	}
}

func TestSparseAxisConsecutiveSaves(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	st := wb.Styles.Get(0)
	st.Fill = "FFFF00"
	sh.Layout.ColStyles[2] = wb.Styles.Intern(st)
	st.Font.Bold = true
	sh.Layout.RowStyles[8] = wb.Styles.Intern(st)
	sh.Set(3, 2, model.StringCell("inherited"))
	sh.Set(10000, 0, model.StringCell("distant"))
	first := filepath.Join(t.TempDir(), "first.xlsx")
	f, err := Save(first, wb, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sh.Set(1, 1, model.StringCell("later edit"))
	second := filepath.Join(t.TempDir(), "second.xlsx")
	_, err = Save(second, wb, f, Changes{Cells: map[*model.Sheet]map[[2]int]bool{sh: {{1, 1}: true}}})
	if err != nil {
		t.Fatal(err)
	}
	got, src, err := Read(second)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	for _, rc := range [][2]int{{3, 2}, {900000, 2}, {8, 10000}} {
		style := got.Styles.Get(got.Sheets[0].EffectiveStyle(rc[0], rc[1]))
		if style.Fill != "FFFF00" {
			t.Fatalf("%v format lost: %+v", rc, style)
		}
	}
	if !got.Styles.Get(got.Sheets[0].EffectiveStyle(8, 10000)).Font.Bold || got.Sheets[0].Get(1, 1).Str != "later edit" {
		t.Fatal("row style or later cell lost")
	}
	z, err := zip.OpenReader(second)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, part := range z.File {
		if strings.HasPrefix(part.Name, "xl/worksheets/sheet") && strings.HasSuffix(part.Name, ".xml") {
			r, err := part.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(io.LimitReader(r, 1<<20))
			r.Close()
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Count(data, []byte("<row ")) > 4 || bytes.Count(data, []byte("<c ")) > 4 {
				t.Fatal("sparse save materialized blank cells")
			}
		}
	}
}
