package xlsxfile

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"

	"ww-excel/internal/model"
)

func sampleWorkbook() *model.Workbook {
	wb := model.NewWorkbook()
	s := wb.Sheets[0]
	s.Name = "銷售"
	st := wb.StyleTable()
	bold := st.Intern(model.Style{Font: model.Font{Name: "Calibri", Size: 14, Bold: true, Color: "C00000"}, Fill: "FFF2CC",
		Border: model.Borders{Bottom: model.BorderLine{Style: "medium", Color: "1F4E79"}},
		Align:  model.Align{H: "center", V: "center", Wrap: true}})
	money := st.Intern(model.Style{Font: model.DefaultFont, NumFmt: "#,##0.00"})
	date := st.Intern(model.Style{Font: model.DefaultFont, NumFmt: "yyyy/m/d"})
	currency := st.Intern(model.Style{Font: model.DefaultFont, NumFmt: `"$"#,##0_);[Red]\("$"#,##0\)`})
	roc := st.Intern(model.Style{Font: model.DefaultFont, NumFmt: `[$-404]e/m/d`})

	s.Set(0, 0, model.Cell{Kind: model.KindString, Str: "標題", S: bold})
	s.Set(1, 0, model.Cell{Kind: model.KindNumber, Num: 1234.5, S: money})
	s.Set(2, 0, model.Cell{Kind: model.KindNumber, Num: 46318, S: date})
	s.Set(3, 0, model.BoolCell(true))
	f := model.NumberCell(1234.5)
	f.F = &model.Formula{Text: "XLOOKUP(1,A2:A3,A2:A3)"}
	s.Set(4, 0, f)
	s.Set(5, 0, model.Cell{S: bold}) // styled but empty
	s.Set(6, 0, model.Cell{Kind: model.KindNumber, Num: 1234, S: currency})
	s.Set(7, 0, model.Cell{Kind: model.KindNumber, Num: 46318, S: roc})
	s.Layout.Cols[0] = model.Size{Px: 140}
	s.Layout.Cols[3] = model.Size{Px: 64, Hidden: true}
	s.Layout.Rows[0] = model.Size{Px: 40}
	s.Layout.Merges = []model.Range{{R0: 0, C0: 0, R1: 0, C1: 2}}
	s.Layout.ColStyles[5] = money

	other := model.NewSheet("Data 2")
	other.Set(0, 0, model.StringCell("x"))
	wb.Sheets = append(wb.Sheets, other)
	return wb
}

func TestRoundTripNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.xlsx")
	wb := sampleWorkbook()
	if _, err := Save(path, wb, nil, Changes{}); err != nil {
		t.Fatal(err)
	}

	// Check the raw file with excelize too.
	raw, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := raw.GetSheetList(); len(got) != 2 || got[0] != "銷售" || got[1] != "Data 2" {
		t.Fatalf("sheets = %v", got)
	}
	if f, _ := raw.GetCellFormula("銷售", "A5"); f != "_xlfn.XLOOKUP(1,A2:A3,A2:A3)" {
		t.Fatalf("stored formula = %q", f)
	}
	if v, _ := raw.GetCellValue("銷售", "A5", excelize.Options{RawCellValue: true}); v != "1234.5" {
		t.Fatalf("cached value = %q", v)
	}
	raw.Close()

	got, src, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	s := got.Sheets[0]
	if c := s.Get(0, 0); c.Str != "標題" {
		t.Fatalf("A1 = %+v", c)
	}
	st := got.Styles.Get(s.Get(0, 0).S)
	if !st.Font.Bold || st.Font.Size != 14 || st.Font.Color != "C00000" || st.Fill != "FFF2CC" ||
		st.Border.Bottom.Style != "medium" || st.Border.Bottom.Color != "1F4E79" || st.Align.H != "center" || !st.Align.Wrap {
		t.Fatalf("A1 style = %+v", st)
	}
	if st := got.Styles.Get(s.Get(1, 0).S); st.NumFmt != "#,##0.00" {
		t.Fatalf("A2 numfmt = %q", st.NumFmt)
	}
	if st := got.Styles.Get(s.Get(2, 0).S); st.NumFmt != "yyyy/m/d" {
		t.Fatalf("A3 numfmt = %q", st.NumFmt)
	}
	if c := s.Get(3, 0); c.Kind != model.KindBool || c.Num != 1 {
		t.Fatalf("A4 = %+v", c)
	}
	// excelize stores cached formula values as t="str", so only the
	// formula is checked; the engine recalculates on load anyway.
	if c := s.Get(4, 0); c.F == nil || c.F.Text != "XLOOKUP(1,A2:A3,A2:A3)" {
		t.Fatalf("A5 = %+v (prefix should be dropped on read)", c)
	}
	for row, want := range map[int]string{6: `"$"#,##0_);[Red]\("$"#,##0\)`, 7: `[$-404]e/m/d`} {
		if got := got.Styles.Get(s.Get(row, 0).S).NumFmt; got != want {
			t.Errorf("row %d numfmt = %q, want %q", row+1, got, want)
		}
	}
	if c := s.Get(5, 0); c.HasContent() || !got.Styles.Get(c.S).Font.Bold {
		t.Fatalf("styled empty A6 = %+v", c)
	}
	l := s.Layout
	if l.ColWidth(0) != 140 || l.RowHeight(0) != 40 || !l.Cols[3].Hidden || len(l.Merges) != 1 || l.Merges[0].C1 != 2 {
		t.Fatalf("layout = %+v", l)
	}
	if got.Styles.Get(l.ColStyles[5]).NumFmt != "#,##0.00" {
		t.Fatal("column style lost")
	}
	if got.Sheets[1].Get(0, 0).Str != "x" || got.Sheets[0].SourceName != "銷售" {
		t.Fatal("second sheet")
	}
}

// A file with features this app does not model: a comment, data
// validation and a rotated cell. Editing one cell must keep them.
func TestInPlaceSavePreservesUnknownContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.xlsx")
	x := excelize.NewFile()
	x.SetCellValue("Sheet1", "A1", "keep")
	x.SetCellValue("Sheet1", "B1", 1)
	x.AddComment("Sheet1", excelize.Comment{Cell: "A1", Author: "QA", Text: "comment survives"})
	dv := excelize.NewDataValidation(true)
	dv.Sqref = "C1:C5"
	dv.SetDropList([]string{"甲", "乙"})
	x.AddDataValidation("Sheet1", dv)
	rot, _ := x.NewStyle(&excelize.Style{Alignment: &excelize.Alignment{TextRotation: 45}})
	x.SetCellStyle("Sheet1", "A1", "A1", rot)
	x.NewSheet("Old")
	x.NewSheet("Gone")
	x.SaveAs(path)
	x.Close()

	wb, src, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	s := wb.Sheets[0]
	old := s.Layout.Clone()
	s.Set(0, 1, model.NumberCell(99)) // edit B1
	s.Layout.Cols[1] = model.Size{Px: 100}
	wb.Sheets[1].Name = "新名稱"                              // rename Old
	wb.Sheets = []*model.Sheet{wb.Sheets[1], wb.Sheets[0]} // drop Gone, reorder
	ch := Changes{
		Cells:      map[*model.Sheet]map[[2]int]bool{s: {{0, 1}: true}},
		OldLayouts: map[*model.Sheet]model.Layout{s: old},
	}
	out := filepath.Join(dir, "out.xlsx")
	if _, err := Save(out, wb, src, ch); err != nil {
		t.Fatal(err)
	}
	src.Close()

	y, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatal(err)
	}
	defer y.Close()
	if got := y.GetSheetList(); len(got) != 2 || got[0] != "新名稱" || got[1] != "Sheet1" {
		t.Fatalf("sheets = %v", got)
	}
	if v, _ := y.GetCellValue("Sheet1", "B1"); v != "99" {
		t.Fatalf("B1 = %q", v)
	}
	if v, _ := y.GetCellValue("Sheet1", "A1"); v != "keep" {
		t.Fatalf("A1 = %q", v)
	}
	comments, _ := y.GetComments("Sheet1")
	if len(comments) != 1 || comments[0].Text != "comment survives" {
		t.Fatalf("comments = %+v", comments)
	}
	dvs, _ := y.GetDataValidations("Sheet1")
	if len(dvs) != 1 {
		t.Fatalf("data validations = %d", len(dvs))
	}
	id, _ := y.GetCellStyle("Sheet1", "A1")
	st, _ := y.GetStyle(id)
	if st.Alignment == nil || st.Alignment.TextRotation != 45 {
		t.Fatalf("untouched cell lost its rotation: %+v", st.Alignment)
	}
	if w, _ := y.GetColWidth("Sheet1", "B"); widthToPx(w) != 100 {
		t.Fatalf("column B width = %v", w)
	}
}

func TestThemeColors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.xlsx")
	x := excelize.NewFile()
	theme := 4 // accent1
	id, _ := x.NewStyle(&excelize.Style{Font: &excelize.Font{ColorTheme: &theme}})
	x.SetCellValue("Sheet1", "A1", "藍")
	x.SetCellStyle("Sheet1", "A1", "A1", id)
	x.SaveAs(path)
	x.Close()

	wb, src, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	color := wb.Styles.Get(wb.Sheets[0].Get(0, 0).S).Font.Color
	if len(color) != 6 || color == "000000" {
		t.Fatalf("theme color not resolved: %q", color)
	}
}

func TestEmptyFormulaResultKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.xlsx")
	x := excelize.NewFile()
	x.SetCellFormula("Sheet1", "B2", `IF(1>2,1,"")`) // no cached value, no style
	x.SetCellValue("Sheet1", "A1", 1)
	x.SaveAs(path)
	x.Close()
	wb, src, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if c := wb.Sheets[0].Get(1, 1); c.F == nil || c.F.Text != `IF(1>2,1,"")` {
		t.Fatalf("B2 = %+v", c)
	}
}
