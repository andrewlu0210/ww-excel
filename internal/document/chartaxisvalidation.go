package document

import (
	"errors"
	"math"
	"ww-excel/internal/model"
)

func validateChartAxes(c *model.Chart) error {
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	for _, a := range []*model.ChartAxis{c.ValueAxis, c.CategoryAxis, c.SecondaryAxis, c.SecondaryCategoryAxis} {
		if a == nil {
			continue
		}
		if a.Minimum != nil && !finite(*a.Minimum) || a.Maximum != nil && !finite(*a.Maximum) {
			return errors.New("座標軸範圍必須是有效數字")
		}
		if a.Minimum != nil && a.Maximum != nil && !finite(*a.Maximum-*a.Minimum) {
			return errors.New("座標軸範圍必須是有效數字")
		}
		if a.Minimum != nil && a.Maximum != nil && *a.Minimum >= *a.Maximum {
			return errors.New("座標軸最小值必須小於最大值")
		}
		if !finite(a.MajorUnit) || a.MajorUnit < 0 {
			return errors.New("主要刻度間距必須大於零或設為自動")
		}
		if !finite(a.FontSize) || a.FontSize != 0 && (a.FontSize < 1 || a.FontSize > 409) {
			return errors.New("座標軸字型大小必須介於 1 與 409")
		}
		if a.LineColor != "" && !chartLabelColor.MatchString(a.LineColor) {
			return errors.New("座標軸色彩設定無效")
		}
		switch a.MajorTick {
		case "", "none", "in", "out", "cross":
		default:
			return errors.New("座標軸刻度線設定無效")
		}
	}
	return nil
}
