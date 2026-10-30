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
	id, _, _, _ := look.style(0, 0, model.Cell{}, headerID)
	if got := wb.StyleTable().Get(id); got != header {
		t.Fatalf("custom header overwritten: %+v", got)
	}
	for row := 1; row <= 3; row++ {
		id, _, _, _ := look.style(row, 0, model.Cell{}, bodyID)
		got := wb.StyleTable().Get(id)
		if got.Fill != body.Fill || got.Font != body.Font {
			t.Fatalf("custom row %d overwritten: %+v", row, got)
		}
		if (got.Border.Bottom.Style == "thin") != (row < 3) {
			t.Fatalf("incorrect row separator: %+v", got.Border)
		}
	}
}
