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

func advancedFillCases() []model.FillSpec {
	out := []model.FillSpec{}
	for _, p := range model.FillPatterns {
		out = append(out, model.FillSpec{Kind: "pattern", Pattern: p, Foreground: "0070C0", Background: "FFF2CC"})
	}
	stops := model.EncodeFillStops([]model.FillStop{{Position: 0, Color: "FF0000"}, {Position: .35, Color: "FFFF00"}, {Position: 1, Color: "0070C0"}})
	out = append(out, model.FillSpec{Kind: "linear", Degree: 37, Stops: stops}, model.FillSpec{Kind: "path", Left: .5, Right: .5, Top: .5, Bottom: .5, Stops: stops})
	return out
}

// Retain all supported patterns and non-preset gradient parameters across
// multiple edits, both file formats, merged cells, and one-step undo/redo.
func TestAdvancedFormatRoundTrip(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	sh.Name = "Advanced"
	cases := advancedFillCases()
	on := true
	diag := &BorderLineDTO{Style: "mediumDashDot", Color: "FF0000"}
	bold := true
	for i, spec := range cases {
		sh.Set(i+2, 1, model.StringCell(spec.Kind+" "+spec.Pattern))
		if err := svc.Format(bg, 0, []RangeDTO{{i + 2, 1, i + 2, 1}}, FormatPatch{FillSpec: &spec, Diagonal: diag, DiagonalUp: &on, DiagonalDown: &on, Bold: &bold}); err != nil {
			t.Fatal(err)
		}
	}
	verify := func(book *model.Workbook) {
		t.Helper()
		s := book.Sheets[0]
		for i, want := range cases {
			st := book.Styles.Get(s.EffectiveStyle(i+2, 1))
			if st.FillSpec != want || st.Border.Diagonal.Style != diag.Style || st.Border.Diagonal.Color != diag.Color || !st.Border.DiagonalUp || !st.Border.DiagonalDown || !st.Font.Bold {
				t.Fatalf("%d got %+v want %+v", i, st, want)
			}
		}
	}
	verify(wb)
	svc.Undo(bg)
	if wb.Styles.Get(sh.EffectiveStyle(len(cases)+1, 1)).FillSpec.Kind != "" {
		t.Fatal("undo advanced style")
	}
	svc.Redo(bg)
	verify(wb)
	var data bytes.Buffer
	if err := wwx.Write(&data, wb); err != nil {
		t.Fatal(err)
	}
	book, err := wwx.Read(&data)
	if err != nil {
		t.Fatal(err)
	}
	verify(book)
	dir := os.Getenv("WW_ADVANCED_FORMAT_OUT")
	if dir == "" {
		dir = t.TempDir()
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	sh.Layout.Cols[1] = model.Size{Px: 280}
	for i := range cases {
		sh.Layout.Rows[i+2] = model.Size{Px: 42}
	}
	ps := model.DefaultPageSetup()
	ps.PrintArea = &model.Range{R1: len(cases) + 2, C1: 3}
	ps.FitWide = 1
	ps.FitTall = 1
	sh.Layout.PageSetup = &ps
	path := filepath.Join(dir, "advanced.xlsx")
	f, err := xlsxfile.Save(path, wb, nil, xlsxfile.Changes{})
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	book, src, err := xlsxfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	verify(book)
	// Editing only the font must retain complete imported fill parameters.
	target := book.Sheets[0]
	c := target.Get(2, 1)
	st := book.Styles.Get(c.S)
	st.Font.Italic = true
	c.S = book.Styles.Intern(st)
	target.Set(2, 1, c)
	f, err = xlsxfile.Save(filepath.Join(dir, "advanced-edited.xlsx"), book, src, xlsxfile.Changes{Cells: map[*model.Sheet]map[[2]int]bool{target: {{2, 1}: true}}})
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	again, src2, err := xlsxfile.Read(filepath.Join(dir, "advanced-edited.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	verify(again)
	src2.Close()
	if native := os.Getenv("WW_ADVANCED_EXCEL_SAVED"); native != "" {
		book, src, err = xlsxfile.Read(native)
		if err != nil {
			t.Fatal(err)
		}
		verify(book)
		src.Close()
	}
	var buf bytes.Buffer
	if err = wwx.Write(&buf, wb); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "advanced.wwx"), buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	pdf, _, err := renderPDF(sess, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "advanced-app.pdf"), pdf, 0644); err != nil {
		t.Fatal(err)
	}
}
func TestAdvancedFormatMixedClearAndValidation(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	sh.Layout.Merges = []model.Range{{R0: 2, C0: 2, R1: 3, C1: 4}}
	spec := advancedFillCases()[0]
	on := true
	if err := svc.Format(bg, 0, []RangeDTO{{3, 3, 3, 3}}, FormatPatch{FillSpec: &spec, Diagonal: &BorderLineDTO{Style: "double", Color: "0070C0"}, DiagonalDown: &on}); err != nil {
		t.Fatal(err)
	}
	st := wb.Styles.Get(sh.EffectiveStyle(2, 2))
	if st.FillSpec != spec || !st.Border.DiagonalDown {
		t.Fatal("merged advanced format")
	}
	summary, err := svc.SelectionFormat(bg, 0, []RangeDTO{{2, 2, 4, 4}})
	if err != nil {
		t.Fatal(err)
	}
	mixed := map[string]bool{}
	for _, k := range summary.Mixed {
		mixed[k] = true
	}
	if !mixed["fillSpec"] || !mixed["diagonalDown"] {
		t.Fatalf("mixed %+v", summary)
	}
	empty := ""
	if err = svc.Format(bg, 0, []RangeDTO{{2, 2, 3, 4}}, FormatPatch{Fill: &empty, Border: "none"}); err != nil {
		t.Fatal(err)
	}
	st = wb.Styles.Get(sh.EffectiveStyle(2, 2))
	if st.HasFill() || st.Border.DiagonalDown {
		t.Fatal("clear advanced format")
	}
	svc.Undo(bg)
	if wb.Styles.Get(sh.EffectiveStyle(2, 2)) != wb.Styles.Get(sh.EffectiveStyle(3, 3)) {
		t.Fatal("merge undo")
	}
	for _, bad := range []model.FillSpec{{Kind: "script"}, {Kind: "pattern", Pattern: "darkGrid", Foreground: "bad", Background: "FFFFFF"}, {Kind: "linear", Stops: `[{"position":-1,"color":"FFFFFF"},{"position":1,"color":"000000"}]`}, {Kind: "path", Left: .8, Right: .2}} {
		if err = svc.Format(bg, 0, []RangeDTO{{}}, FormatPatch{FillSpec: &bad}); err == nil {
			t.Fatal("invalid fill accepted")
		}
	}
}

func TestAdvancedFormatWholeAxesAndDisplayedPrecedence(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	spec := advancedFillCases()[0]
	on := true
	if err := svc.Format(bg, 0, []RangeDTO{{0, 8, model.MaxRows - 1, 8}}, FormatPatch{FillSpec: &spec, Diagonal: &BorderLineDTO{Style: "thin"}, DiagonalUp: &on}); err != nil {
		t.Fatal(err)
	}
	if st := wb.Styles.Get(sh.EffectiveStyle(model.MaxRows-1, 8)); st.FillSpec != spec || !st.Border.DiagonalUp || sh.BlockCount() > 1 {
		t.Fatal("sparse advanced whole column")
	}
	sh.Layout.Tables = []model.Table{{Name: "Colors", Style: "TableStyleMedium2", Range: model.Range{R0: 0, C0: 7, R1: 3, C1: 9}}}
	sh.Set(1, 8, model.NumberCell(1))
	look := newLook(sess.doc, 0)
	id, _, _, _, _ := look.style(1, 8, sh.Get(1, 8), sh.EffectiveStyle(1, 8))
	if wb.Styles.Get(id).FillSpec != spec {
		t.Fatal("table overrides advanced direct fill")
	}
	sh.Layout.CondFormats = []model.CFRule{{Type: "formula", Ranges: []model.Range{{R0: 1, C0: 8, R1: 1, C1: 8}}, Value1: "TRUE", Format: model.CFFormat{Fill: "FFFFFF"}}}
	look = newLook(sess.doc, 0)
	id, _, _, _, _ = look.style(1, 8, sh.Get(1, 8), sh.EffectiveStyle(1, 8))
	st := wb.Styles.Get(id)
	if st.Fill != "FFFFFF" || st.FillSpec.Kind != "" {
		t.Fatal("conditional fill must override advanced direct fill")
	}
}
