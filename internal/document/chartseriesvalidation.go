package document

import (
	"errors"
	"math"
	"ww-excel/internal/model"
)

func validateChartSeriesStyle(c *model.Chart) error {
	if c.GapWidth != nil && (math.IsNaN(*c.GapWidth) || math.IsInf(*c.GapWidth, 0) || *c.GapWidth < 0 || *c.GapWidth > 500 || math.Trunc(*c.GapWidth) != *c.GapWidth) {
		return errors.New("柱距必須介於 0 與 500")
	}
	if c.HoleSize != 0 && (c.HoleSize < 10 || c.HoleSize > 90) {
		return errors.New("環圈內徑必須介於 10 與 90")
	}
	for i, s := range c.Series {
		unchangedWidth := c.Source != nil && c.Source.Baseline != nil && i < len(c.Source.Baseline.Series) && s.LineWidth == c.Source.Baseline.Series[i].LineWidth
		if s.Color != "" && !chartLabelColor.MatchString(s.Color) {
			return errors.New("數列色彩設定無效")
		}
		if math.IsNaN(s.LineWidth) || math.IsInf(s.LineWidth, 0) || !unchangedWidth && s.LineWidth != 0 && (s.LineWidth < .25 || s.LineWidth > 30) {
			return errors.New("折線粗細必須介於 0.25 與 30")
		}
		switch s.Marker {
		case "", "auto", "circle", "square", "diamond", "triangle", "x", "plus", "star", "dash", "dot", "none", "picture":
		default:
			return errors.New("資料點標記設定無效")
		}
	}
	return nil
}
