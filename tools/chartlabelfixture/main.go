// Generate a workbook for interactive chart label editing checks.
package main

import (
	"log"
	"os"
	"strconv"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func main() {
	filename := "test_data/chart-labels.xlsx"
	if len(os.Args) > 1 {
		filename = os.Args[1]
	}
	d := document.New()
	sh := d.Workbook().Sheets[0]
	sh.Name = "圖表標籤驗收"
	for r, row := range [][]string{{"月份", "收入"}, {"一月", "100"}, {"二月", "150"}, {"三月", "-50"}} {
		for c, value := range row {
			if n, err := strconv.ParseFloat(value, 64); err == nil {
				sh.Set(r, c, model.NumberCell(n))
			} else {
				sh.Set(r, c, model.StringCell(value))
			}
		}
	}
	sh.Layout.Objects = []model.Object{{ID: 1, Kind: "chart", Anchor: model.Anchor{Row: 1, Col: 3}, W: 640, H: 360, Chart: &model.Chart{Type: "column", Title: "銷售資料", Legend: "right", Series: []model.ChartSeries{{Name: "'圖表標籤驗收'!$B$1", Categories: "'圖表標籤驗收'!$A$2:$A$4", Values: "'圖表標籤驗收'!$B$2:$B$4"}}}}}
	if err := d.SaveAs(filename); err != nil {
		log.Fatal(err)
	}
	log.Print(filename)
}
