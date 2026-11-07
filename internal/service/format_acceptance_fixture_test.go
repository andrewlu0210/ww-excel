package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
)

// Set WW_FORMAT_ACCEPTANCE_OUT to retain synthetic files for native Excel QA.
func TestFormatAcceptanceFixture(t *testing.T) {
	dir := os.Getenv("WW_FORMAT_ACCEPTANCE_OUT")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	sh.Name = "Formats"
	ps := model.DefaultPageSetup()
	ps.PrintArea = &model.Range{R1: 20, C1: 7}
	ps.FitWide = 1
	ps.FitTall = 1
	sh.Layout.PageSetup = &ps
	for c := 0; c < 8; c++ {
		sh.Layout.Cols[c] = model.Size{Px: 80}
	}
	apply := func(r RangeDTO, p FormatPatch) {
		t.Helper()
		if err := svc.Format(bg, 0, []RangeDTO{r}, p); err != nil {
			t.Fatal(err)
		}
	}
	text := func(r, c int, s string) { sh.Set(r, c, model.StringCell(s)) }
	fill, size, bold, font, num := "FFFF00", 14.0, true, "Arial", "#,##0.00"
	text(0, 0, "Format acceptance / native Excel")
	sh.Set(3, 1, model.NumberCell(1234.5))
	apply(RangeDTO{3, 1, 3, 1}, FormatPatch{Fill: &fill, FontSize: &size, Bold: &bold, FontName: &font, NumFmt: &num})
	sh.Layout.Merges = []model.Range{{R0: 7, C0: 0, R1: 7, C1: 2}}
	text(7, 0, "Merged via B8")
	fill = "DDEBF7"
	size = 20
	apply(RangeDTO{7, 1, 7, 1}, FormatPatch{Fill: &fill, FontSize: &size, AutoRows: []SizeDTO{{I: 7, Px: 36}}})
	red := &BorderLineDTO{Style: "double", Color: "FF0000"}
	blue := &BorderLineDTO{Style: "mediumDashDot", Color: "0070C0"}
	for r := 3; r <= 5; r++ {
		for c := 3; c <= 5; c++ {
			text(r, c, "grid")
		}
	}
	apply(RangeDTO{3, 3, 5, 5}, FormatPatch{Borders: &BorderPatchDTO{Top: red, Bottom: red, Left: red, Right: red, InsideH: blue, InsideV: blue}})
	for i, color := range []string{"", "FFFFFF", "FFC000"} {
		text(10+i, 1, []string{"No fill", "White", "Orange"}[i])
		apply(RangeDTO{10 + i, 1, 10 + i, 1}, FormatPatch{Fill: &color})
	}
	styles := []string{"thin", "medium", "dashed", "dotted", "thick", "double", "hair", "mediumDashed", "dashDot", "mediumDashDot", "dashDotDot", "mediumDashDotDot", "slantDashDot"}
	for i, style := range styles {
		text(i+1, 6, style)
		apply(RangeDTO{i + 1, 6, i + 1, 7}, FormatPatch{Borders: &BorderPatchDTO{Bottom: &BorderLineDTO{Style: style, Color: "0070C0"}}})
	}
	var ww bytes.Buffer
	if err := wwx.Write(&ww, wb); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "formats.wwx"), ww.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := xlsxfile.Save(filepath.Join(dir, "formats.xlsx"), wb, nil, xlsxfile.Changes{})
	if err != nil {
		t.Fatal(err)
	}
	props, err := f.GetSheetProps(sh.Name)
	if err != nil || props.FitToPage == nil || !*props.FitToPage {
		t.Fatal("Excel fit-to-page flag missing", err)
	}
	f.Close()
	pdf, _, err := renderPDF(sess, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "formats-app.pdf"), pdf, 0644); err != nil {
		t.Fatal(err)
	}
	if native := os.Getenv("WW_FORMAT_EXCEL_SAVED"); native != "" {
		book, src, err := xlsxfile.Read(native)
		if err != nil {
			t.Fatal(err)
		}
		defer src.Close()
		s := book.Sheets[0]
		at := func(r, c int) model.Style { return book.Styles.Get(s.EffectiveStyle(r, c)) }
		if at(3, 1).Fill != "FFFF00" || at(3, 1).Font.Size != 14 || !at(3, 1).Font.Bold || at(3, 1).NumFmt != "#,##0.00" || s.Get(3, 1).Num != 1234.5 {
			t.Fatalf("native Excel number/font/fill changed: %+v", at(3, 1))
		}
		if at(7, 0).Fill != "DDEBF7" || at(7, 0).Font.Size != 20 || len(s.Layout.Merges) != 1 {
			t.Fatal("native merge changed")
		}
		if at(10, 1).Fill != "" || at(11, 1).Fill != "FFFFFF" || at(12, 1).Fill != "FFC000" {
			t.Fatal("native fill changed")
		}
		for i, style := range styles {
			line := at(i+1, 6).Border.Bottom
			if line.Style != style || line.Color != "0070C0" {
				t.Fatalf("native %s: %+v", style, line)
			}
		}
	}
}

// The optional input is the desktop acceptance workbook resaved by native Excel.
func TestFormatAcceptanceNativeAxes(t *testing.T) {
	path := os.Getenv("WW_FORMAT_EXCEL_AXES")
	if path == "" {
		t.Skip("native Excel axis workbook not supplied")
	}
	wb, src, err := xlsxfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	sh := wb.Sheets[0]
	for _, tc := range []struct {
		r, c int
		fill string
	}{{1048575, 14, "FFFF00"}, {900000, 14, "FFFF00"}, {145, 16383, "00B050"}, {145, 14, "00B050"}, {140, 9, "FFC000"}} {
		st := wb.Styles.Get(sh.EffectiveStyle(tc.r, tc.c))
		if st.Fill != tc.fill {
			t.Fatalf("native axis (%d,%d): %+v", tc.r, tc.c, st)
		}
	}
	st := wb.Styles.Get(sh.EffectiveStyle(900000, 14))
	if st.Font.Name != "Arial" || !st.Font.Italic {
		t.Fatal("native inherited font lost")
	}
	found := false
	for _, m := range sh.Layout.Merges {
		if m.R0 == 140 && m.C0 == 9 && m.C1 == 11 {
			found = true
		}
	}
	if !found {
		t.Fatal("native merged format lost")
	}
}
