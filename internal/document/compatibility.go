package document

import (
	"fmt"
	"reflect"

	"ww-excel/internal/i18n"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
)

// SaveRisk describes an impact of the selected output format and current edits.
// It is deliberately limited to behavior known from the writers.
type SaveRisk struct {
	Code     string
	Sheet    string
	ObjectID int
	Message  string
}

func (d *Document) SaveRisks(path string) []SaveRisk {
	var risks []SaveRisk
	add := func(code, sheet string, id int, message string) {
		risks = append(risks, SaveRisk{code, sheet, id, message})
	}
	switch FormatForPath(path) {
	case FormatCSV:
		if len(d.wb.Sheets) == 0 {
			return nil
		}
		sh := d.wb.Sheets[0]
		if len(d.wb.Sheets) > 1 {
			add("csv-sheets", sh.Name, 0, fmt.Sprintf(i18n.T("CSV 只保存第一張工作表「%s」；其餘 %d 張工作表不會寫入檔案。"), sh.Name, len(d.wb.Sheets)-1))
		}
		formula, formatting := false, false
		sh.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			formula = formula || c.F != nil
			formatting = formatting || c.Rich != nil || c.S != 0
		})
		if formula {
			add("csv-formulas", sh.Name, 0, fmt.Sprintf(i18n.T("工作表「%s」的公式只會保存目前計算結果，公式本身不會保留。"), sh.Name))
		}
		l := sh.Layout
		if formatting || len(l.RowStyles)+len(l.ColStyles)+len(l.Merges)+len(l.Rows)+len(l.Cols) > 0 || l.DefaultRow != model.DefaultRowHeight || l.DefaultCol != model.DefaultColWidth || !reflect.DeepEqual(d.wb.StyleTable().Default(), model.NewWorkbook().StyleTable().Default()) {
			add("csv-formatting", sh.Name, 0, fmt.Sprintf(i18n.T("工作表「%s」的字型、色彩、數值格式、合併儲存格與欄寬列高不會保留；日期與百分比會輸出為底層數值。"), sh.Name))
		}
		if len(l.Objects) > 0 {
			add("csv-objects", sh.Name, 0, fmt.Sprintf(i18n.T("工作表「%s」的圖表、圖片與圖形不會寫入 CSV。"), sh.Name))
		}
		if len(l.Notes)+len(l.Validations)+len(l.CondFormats)+len(l.Tables) > 0 || l.Filter != nil || l.PageSetup != nil || l.FreezeRows != 0 || l.FreezeCols != 0 || l.HideGridLines || len(d.wb.Names) > 0 {
			add("csv-settings", sh.Name, 0, fmt.Sprintf(i18n.T("工作表「%s」的註解、資料規則、表格、篩選、凍結窗格、列印設定與具名範圍不會保留。"), sh.Name))
		}
	case FormatXLSX:
		if d.xlsx == nil {
			return nil
		}
		for _, sh := range d.wb.Sheets {
			if !d.layoutDirty[sh] || sh.SourceName == "" {
				continue
			}
			old := d.layoutBase[sh].Objects
			for _, o := range xlsxfile.RewrittenCharts(old, sh.Layout.Objects) {
				if !d.importedCharts[sh][o.ID] {
					continue
				}
				for _, prev := range old {
					if prev.ID != o.ID || prev.Chart == nil {
						continue
					}
					title := o.Chart.Title
					if title == "" {
						title = i18n.T("未命名圖表")
					}
					add("xlsx-chart", sh.Name, o.ID, fmt.Sprintf(i18n.T("工作表「%s」的圖表「%s」（物件 %d）會重新寫入，原檔中尚未支援的進階格式可能簡化。"), sh.Name, title, o.ID))
					break
				}
			}
		}
	}
	return risks
}
