package document

import (
	"errors"
	"math"
	"regexp"
	"ww-excel/internal/model"
)

var chartLabelColor = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

func validateChartLabels(l *model.ChartLabels) error {
	if l == nil {
		return nil
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	size := func(v float64) bool { return finite(v) && (v == 0 || v >= 1 && v <= 409) }
	color := func(v string) bool { return v == "" || chartLabelColor.MatchString(v) }
	position := func(v string) bool {
		switch v {
		case "", "ctr", "inEnd", "inBase", "outEnd", "t", "b", "l", "r", "bestFit":
			return true
		}
		return false
	}
	if !size(l.FontSize) || !finite(l.Rotation) || l.Rotation < -90 || l.Rotation > 90 || !color(l.Color) || !color(l.Fill) || !color(l.Border) || !position(l.Position) {
		return errors.New("資料標籤的字型、色彩、角度或位置設定無效")
	}
	if len(l.Points) > 10000 || len(l.Separator) > 100 {
		return errors.New("資料標籤的設定過大")
	}
	for index, p := range l.Points {
		if index < 0 || index >= 1000000 || !size(p.FontSize) || !color(p.Color) || !position(p.Position) {
			return errors.New("個別資料標籤的設定無效")
		}
		if p.CustomText != nil && len(*p.CustomText) > 32768 {
			return errors.New("資料標籤文字過長")
		}
		if p.Layout != nil {
			v := p.Layout
			if !finite(v.X) || !finite(v.Y) || !finite(v.W) || !finite(v.H) || math.Abs(v.X) > 2 || math.Abs(v.Y) > 2 || v.W < 0 || v.W > 2 || v.H < 0 || v.H > 2 {
				return errors.New("資料標籤的自訂位置必須介於允許範圍")
			}
			for _, mode := range []string{v.XMode, v.YMode} {
				if mode != "" && mode != "edge" && mode != "factor" {
					return errors.New("資料標籤的位置模式無效")
				}
			}
		}
	}
	return nil
}
