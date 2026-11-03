// Generate an Excel workbook for all conditional icon sets and their display modes.
package main

import (
	"flag"
	"log"

	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

var sets = []string{"3Arrows", "3ArrowsGray", "3Flags", "3TrafficLights1", "3TrafficLights2", "3Signs", "3Symbols", "3Symbols2", "4Arrows", "4ArrowsGray", "4RedToBlack", "4Rating", "4TrafficLights", "5Arrows", "5ArrowsGray", "5Rating", "5Quarters", "3Stars", "3Triangles", "5Boxes"}

func main() {
	out := flag.String("o", "test_data/conditional-icons.xlsx", "output .xlsx or .wwx path")
	flag.Parse()
	wb := model.NewWorkbook()
	gallery := wb.Sheets[0]
	gallery.Name = "條件格式圖示驗收"
	gallery.Layout.HideGridLines = true
	gallery.Layout.DefaultRow = 26
	gallery.Layout.Cols[0] = model.Size{Px: 180}
	gallery.Layout.Cols[6] = model.Size{Px: 20}
	for c := 1; c < 6; c++ {
		gallery.Layout.Cols[c] = model.Size{Px: 55}
	}
	for c := 7; c < 12; c++ {
		gallery.Layout.Cols[c] = model.Size{Px: 75}
	}
	header := wb.StyleTable().Intern(model.Style{Font: model.Font{Name: "Calibri", Size: 11, Bold: true, Color: "FFFFFF"}, Fill: "33704B"})
	body := wb.StyleTable().Intern(model.Style{Font: model.Font{Name: "Calibri", Size: 11}, NumFmt: "0"})
	setText := func(sh *model.Sheet, row, col int, text string, style uint32) {
		cell := model.StringCell(text)
		cell.S = style
		sh.Set(row, col, cell)
	}
	setNumber := func(sh *model.Sheet, row, col int, value float64) {
		cell := model.NumberCell(value)
		cell.S = body
		sh.Set(row, col, cell)
	}
	for c := 0; c < 12; c++ {
		setText(gallery, 0, c, "", header)
	}
	setText(gallery, 0, 0, "圖示集", header)
	setText(gallery, 0, 1, "只顯示圖示", header)
	setText(gallery, 0, 7, "圖示＋數值", header)
	for i, set := range sets {
		row, n := i+1, model.IconSetSize(set)
		setText(gallery, row, 0, set, body)
		for j := 0; j < n; j++ {
			v := float64(j) * 100 / float64(n-1)
			setNumber(gallery, row, j+1, v)
			setNumber(gallery, row, j+7, v)
		}
		for _, start := range []int{1, 7} {
			gallery.Layout.CondFormats = append(gallery.Layout.CondFormats, model.CFRule{Type: "icon_set", Ranges: []model.Range{{R0: row, R1: row, C0: start, C1: start + n - 1}}, IconSet: &model.CFIconSet{Set: set, HideValue: start == 1, Thresholds: model.IconThresholds(n)}})
		}
	}
	detail := model.NewSheet("門檻與自訂")
	detail.Layout.DefaultRow = 32
	detail.Layout.Cols[0] = model.Size{Px: 190}
	detail.Layout.DefaultCol = 90
	wb.Sheets = append(wb.Sheets, detail)
	type example struct {
		name   string
		values []float64
		rule   *model.CFIconSet
	}
	numeric := func() *model.CFIconSet {
		return &model.CFIconSet{Set: "3Arrows", Thresholds: []model.CFThreshold{{Type: "num", Value: "0"}, {Type: "num", Value: "33"}, {Type: "num", Value: "67"}}}
	}
	reverse := numeric()
	reverse.Reverse = true
	strict := numeric()
	strict.Thresholds[1] = model.CFThreshold{Type: "num", Value: "50", Strict: true}
	strict.Thresholds[2] = model.CFThreshold{Type: "num", Value: "90", Strict: true}
	custom := numeric()
	custom.HideValue = true
	custom.Icons = []model.CFIcon{{Set: "3Flags", Index: 0}, {Set: "NoIcons"}, {Set: "5Rating", Index: 4}}
	customValues := *custom
	customValues.HideValue = false
	percentile := numeric()
	percentile.Thresholds[1] = model.CFThreshold{Type: "percentile", Value: "40"}
	percentile.Thresholds[2] = model.CFThreshold{Type: "percentile", Value: "80"}
	formula := numeric()
	formula.Thresholds[1] = model.CFThreshold{Type: "formula", Value: "$B$10"}
	formula.Thresholds[2] = model.CFThreshold{Type: "formula", Value: "$C$10"}
	examples := []example{{"反向箭頭", []float64{20, 50, 90}, reverse}, {"嚴格門檻", []float64{49, 50, 51, 90, 91}, strict}, {"自訂與隱藏", []float64{20, 50, 90}, custom}, {"自訂含數值", []float64{20, 50, 90}, &customValues}, {"百分位數", []float64{0, 20, 50, 90, 100}, percentile}, {"公式門檻", []float64{20, 50, 90}, formula}}
	setText(detail, 0, 0, "門檻與自訂圖示", header)
	for i, e := range examples {
		row := i + 1
		setText(detail, row, 0, e.name, body)
		for c, v := range e.values {
			setNumber(detail, row, c+1, v)
		}
		detail.Layout.CondFormats = append(detail.Layout.CondFormats, model.CFRule{Type: "icon_set", Ranges: []model.Range{{R0: row, R1: row, C0: 1, C1: len(e.values)}}, IconSet: e.rule})
	}
	setNumber(detail, 9, 1, 40)
	setNumber(detail, 9, 2, 80)
	narrow := model.NewSheet("窄欄與合併")
	narrow.Layout.DefaultRow = 32
	narrow.Layout.Cols[0] = model.Size{Px: 180}
	narrow.Layout.Cols[1] = model.Size{Px: 20}
	narrow.Layout.Cols[2] = model.Size{Px: 20}
	narrow.Layout.DefaultCol = 80
	wb.Sheets = append(wb.Sheets, narrow)
	setText(narrow, 0, 0, "窄欄與合併", header)
	narrow.Layout.Merges = []model.Range{{R0: 1, R1: 1, C0: 3, C1: 4}}
	for _, c := range []int{1, 2, 3} {
		setNumber(narrow, 1, c, 90)
		set := numeric()
		set.HideValue = c == 1
		narrow.Layout.CondFormats = append(narrow.Layout.CondFormats, model.CFRule{Type: "icon_set", Ranges: []model.Range{{R0: 1, R1: 1, C0: c, C1: c}}, IconSet: set})
	}
	d := document.FromWorkbook(wb, "")
	defer d.Close()
	if err := d.SaveAs(*out); err != nil {
		log.Fatal(err)
	}
	log.Print(*out)
}
