package document

import (
	"errors"
	"math"
	"strings"
	"ww-excel/internal/command"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// FormatChartLabels shares Excel number formatting with cells and axis ticks.
func FormatChartLabels(labels *model.ChartLabels, values []*float64, name string, categories []string) []string {
	if labels == nil || labels.Disabled {
		return nil
	}
	total := 0.0
	for _, v := range values {
		if v != nil && !math.IsNaN(*v) && !math.IsInf(*v, 0) && *v > 0 {
			total += *v
		}
	}
	out := make([]string, len(values))
	separator := labels.Separator
	if separator == "" {
		separator = "\n"
	}
	flag := func(base bool, override *bool) bool {
		if override != nil {
			return *override
		}
		return base
	}
	for i, v := range values {
		point := labels.Points[i]
		if point.Hidden {
			continue
		}
		if point.CustomText != nil {
			out[i] = *point.CustomText
			continue
		}
		if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
			continue
		}
		parts := []string{}
		if flag(labels.ShowSeries, point.ShowSeries) && name != "" {
			parts = append(parts, name)
		}
		if flag(labels.ShowCategory, point.ShowCategory) && i < len(categories) && categories[i] != "" {
			parts = append(parts, categories[i])
		}
		format := labels.Format
		if point.Format != "" {
			format = point.Format
		}
		if flag(labels.ShowValue, point.ShowValue) {
			parts = append(parts, numfmt.Format(*v, format))
		}
		if flag(labels.ShowPercent, point.ShowPercent) && total > 0 {
			format := labels.PercentFormat
			if format == "" {
				format = "0%"
			}
			parts = append(parts, numfmt.Format(*v/total, format))
		}
		out[i] = strings.Join(parts, separator)
	}
	return out
}

func (d *Document) ChartLabelTexts(sheet int, c *model.Chart, data ChartData, index int) []string {
	if index >= len(c.Series) || index >= len(data.Series) || c.Series[index].Labels == nil {
		return nil
	}
	labels := *c.Series[index].Labels
	labels.Points = map[int]model.ChartLabelPoint{}
	for key, point := range c.Series[index].Labels.Points {
		if point.Reference != "" {
			texts := d.chartTexts(sheet, point.Reference, func(sh *model.Sheet, row, col int, cell model.Cell) string {
				if cell.Kind == model.KindNumber {
					style := cell.S
					if style == 0 {
						style = sh.Layout.StyleAt(row, col)
					}
					return numfmt.Format(cell.Num, d.wb.StyleTable().Get(style).NumFmt)
				}
				return cell.Display()
			})
			if len(texts) > 0 {
				text := strings.Join(texts, "\n")
				point.CustomText = &text
			}
		}
		labels.Points[key] = point
	}
	return FormatChartLabels(&labels, data.Series[index].Values, data.Series[index].Name, data.Categories)
}

// MoveChartLabelCommand updates one label without replacing concurrent chart edits.
func (d *Document) MoveChartLabelCommand(sheet, id, series, point int, x, y float64) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	object := d.wb.Sheets[sheet].Layout.ObjectByID(id)
	if object == nil || object.Chart == nil || object.ReadOnly {
		return nil, errors.New("圖表不存在或無法編輯")
	}
	c := object.Chart
	if series < 0 || series >= len(c.Series) || point < 0 || point >= len(d.chartNumbers(sheet, c.Series[series].Values)) {
		return nil, errors.New("圖表資料點不存在")
	}
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || x < -2 || x > 2 || y < -2 || y > 2 {
		return nil, errors.New("資料標籤的自訂位置必須介於允許範圍")
	}
	next := *object
	next.Chart = c.Clone()
	labels := next.Chart.Series[series].Labels
	if labels == nil {
		if c.Type != "pie" && c.Type != "doughnut" {
			return nil, errors.New("請先啟用資料標籤")
		}
		labels = &model.ChartLabels{Format: "General", ShowPercent: true, FontSize: 8.25, Color: "ffffff"}
		next.Chart.Series[series].Labels = labels
	}
	p := labels.Points[point]
	if labels.Disabled || p.Hidden {
		return nil, errors.New("無法移動隱藏的資料標籤")
	}
	layout := model.ChartLabelLayout{X: x, Y: y, XMode: "edge", YMode: "edge"}
	if p.Layout != nil {
		layout.W = p.Layout.W
		layout.H = p.Layout.H
	}
	p.Center = false
	p.Layout = &layout
	if labels.Points == nil {
		labels.Points = map[int]model.ChartLabelPoint{}
	}
	labels.Points[point] = p
	return d.UpdateObjectCommand(sheet, next, "移動資料標籤")
}
