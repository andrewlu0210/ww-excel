package service

import (
	"testing"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func TestUnknownTableStylePreservesDirectFormatting(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	header := model.Style{Font: model.Font{Color: "0592FE"}, Fill: "D8D8D1"}
	body := model.Style{Font: model.Font{Color: "60605E"}, Fill: "D8D8D1"}
	headerID := wb.StyleTable().Intern(header)
	bodyID := wb.StyleTable().Intern(body)
	sh.Layout.Tables = []model.Table{{Name: "Income", Style: "Custom budget", Range: model.Range{R0: 0, C0: 0, R1: 3, C1: 1}, RowBorder: model.BorderLine{Style: "thin", Color: "B8B8AB"}}}
	look := newLook(document.FromWorkbook(wb, ""), 0)
	id, _, _, _, _ := look.style(0, 0, model.Cell{}, headerID)
	if got := wb.StyleTable().Get(id); got != header {
		t.Fatalf("custom header overwritten: %+v", got)
	}
	for row := 1; row <= 3; row++ {
		id, _, _, _, _ := look.style(row, 0, model.Cell{}, bodyID)
		got := wb.StyleTable().Get(id)
		if got.Fill != body.Fill || got.Font != body.Font {
			t.Fatalf("custom row %d overwritten: %+v", row, got)
		}
		if (got.Border.Bottom.Style == "thin") != (row < 3) {
			t.Fatalf("incorrect row separator: %+v", got.Border)
		}
	}
}

func TestCustomTableFontScopesAndConditionalPriority(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	blue, gray, black, red := "1C639E", "808080", "262626", "FF0000"
	on, off := true, false
	sh.Layout.Tables = []model.Table{{Name: "Tasks", Style: "Custom", Range: model.Range{R0: 5, C0: 1, R1: 9, C1: 3}, FirstColumn: true, Appearance: &model.TableAppearance{
		Whole: &model.RichFont{Color: &blue, Bold: &off}, First: &model.RichFont{Color: &gray}, Header: &model.RichFont{Color: &black, Bold: &on}, HeaderBorder: model.BorderLine{Style: "medium", Color: "000000"},
	}}}
	baseStyle := model.Style{Font: model.Font{Name: "Arial", Size: 10, Color: gray, Bold: true}, NumFmt: "yyyy/m/d"}
	base := wb.StyleTable().Intern(baseStyle)
	d := document.FromWorkbook(wb, "")
	for _, tc := range []struct {
		row, col int
		color    string
		bold     bool
	}{{5, 1, black, true}, {5, 2, black, true}, {6, 1, gray, false}, {6, 2, blue, false}, {9, 3, blue, false}} {
		id, _, _, _, _ := newLook(d, 0).style(tc.row, tc.col, model.Cell{}, base)
		got := wb.StyleTable().Get(id)
		if got.Font.Color != tc.color || got.Font.Bold != tc.bold || got.Font.Name != "Arial" || got.NumFmt != baseStyle.NumFmt {
			t.Fatalf("row %d col %d: %+v", tc.row, tc.col, got)
		}
		if tc.row == 5 && got.Border.Bottom.Style != "medium" {
			t.Fatal("header border missing")
		}
	}
	sh.Layout.CondFormats = []model.CFRule{{Type: "formula", Ranges: []model.Range{{R0: 6, C0: 2, R1: 6, C1: 2}}, Value1: "TRUE", Format: model.CFFormat{Color: red}}}
	id, _, _, _, _ := newLook(d, 0).style(6, 2, model.StringCell("item"), base)
	if got := wb.StyleTable().Get(id).Font.Color; got != red {
		t.Fatalf("conditional format lost priority: %s", got)
	}
}
