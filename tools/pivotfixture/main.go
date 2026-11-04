// Generate a deterministic workbook for interactive pivot editing checks.
package main

import (
	"log"
	"os"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func main() {
	path := "test_data/pivot-edit.xlsx"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	d := document.New()
	sh := d.Workbook().Sheets[0]
	sh.Name = "樞紐分析表驗收"
	data := [][]any{{"產品", "銷售人員", "金額"}, {"啤酒", "甲", 1400.0}, {"啤酒", "乙", 750.0}, {"紅酒", "甲", 1600.0}, {"紅酒", "乙", 1000.0}, {"汽水", "甲", 690.0}, {"汽水", "乙", 510.0}}
	for r, row := range data {
		for c, v := range row {
			if s, ok := v.(string); ok {
				sh.Set(r, c, model.StringCell(s))
			} else {
				sh.Set(r, c, model.NumberCell(v.(float64)))
			}
		}
	}
	cmd, name, err := d.CreateTableCommand(0, model.Range{R0: 0, C0: 0, R1: 6, C1: 2}, true, "")
	if err != nil {
		log.Fatal(err)
	}
	d.Exec(cmd)
	st := d.Workbook().StyleTable().Default()
	st.Fill = "359966"
	st.Font.Color = "FFFFFF"
	st.Font.Bold = true
	style := d.Workbook().StyleTable().InternExplicit(st)
	p := model.Pivot{Name: "SalesPivot", Source: name, Rows: []string{"產品"}, Values: []model.PivotValue{{Field: "金額", Function: "sum", Caption: "加總 - 金額", Format: `"NT$"#,##0`}}, RowTotals: true, ColTotals: true, Range: model.Range{R0: 10, C0: 4, R1: 10, C1: 4}, HeaderStyle: style, TotalStyle: style}
	sh.Layout.Pivots = []model.Pivot{p}
	cmd, err = d.PivotCommand(0, p)
	if err != nil {
		log.Fatal(err)
	}
	d.Exec(cmd)
	sh.Layout.Cols[0] = model.Size{Px: 100}
	sh.Layout.Cols[1] = model.Size{Px: 100}
	sh.Layout.Cols[4] = model.Size{Px: 150}
	sh.Layout.Cols[5] = model.Size{Px: 150}
	if err := d.SaveAs(path); err != nil {
		log.Fatal(err)
	}
	log.Print(path)
}
