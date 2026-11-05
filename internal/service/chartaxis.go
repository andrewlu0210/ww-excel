package service

import (
	"math"

	"ww-excel/internal/document"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

type ChartScaleDTO struct {
	Min    float64  `json:"min"`
	Max    float64  `json:"max"`
	Step   float64  `json:"step"`
	Labels []string `json:"labels"`
}

// Format ticks in Go so axes use the same Excel number formats as cells.
func chartValueScale(c *model.Chart, data document.ChartData) *ChartScaleDTO {
	if c.Type == "scatter" {
		return chartScatterScale(c, data, false)
	}
	return chartAxisScale(c, data, false)
}

func chartSecondaryScale(c *model.Chart, data document.ChartData) *ChartScaleDTO {
	if c.Type != "combo" {
		return nil
	}
	return chartAxisScale(c, data, true)
}

func chartAxisScale(c *model.Chart, data document.ChartData, secondary bool) *ChartScaleDTO {
	if c.Type == "combo" {
		filtered := document.ChartData{}
		copyChart := *c
		copyChart.Type = "line"
		copyChart.Series = nil
		if secondary {
			copyChart.ValueAxis = c.SecondaryAxis
		}
		for i, series := range c.Series {
			if series.Secondary != secondary || i >= len(data.Series) {
				continue
			}
			filtered.Series = append(filtered.Series, data.Series[i])
			copyChart.Series = append(copyChart.Series, series)
			if series.Type == "column" {
				copyChart.Type = "column"
			}
		}
		if len(filtered.Series) == 0 {
			return nil
		}
		c, data = &copyChart, filtered
	}

	if c.ValueAxis == nil || c.Type == "pie" || c.Type == "doughnut" || c.Type == "scatter" {
		return nil
	}
	a := c.ValueAxis
	lo, hi := math.Inf(1), math.Inf(-1)
	n := 0
	for _, s := range data.Series {
		n = max(n, len(s.Values))
	}
	for i := 0; i < n; i++ {
		pos, neg := 0.0, 0.0
		for si, s := range data.Series {
			if i >= len(s.Values) || s.Values[i] == nil {
				continue
			}
			v := *s.Values[i]
			if math.IsNaN(v) || math.IsInf(v, 0) {
				continue
			}
			if c.Stacked && c.Type != "line" && (si >= len(c.Series) || c.Series[si].Type != "line") {
				if v >= 0 {
					pos += v
				} else {
					neg += v
				}
			} else {
				lo = math.Min(lo, v)
				hi = math.Max(hi, v)
			}
		}
		if c.Stacked && c.Type != "line" {
			lo = math.Min(lo, neg)
			hi = math.Max(hi, pos)
		}
	}
	if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		lo, hi = 0, 1
	}
	if c.Type != "line" || (lo > 0 && lo < hi*0.6) || (hi < 0 && hi > lo*0.6) {
		lo = math.Min(lo, 0)
		hi = math.Max(hi, 0)
	}
	if lo == hi {
		lo = math.Min(0, lo)
		hi = math.Max(0, hi)
		if lo == hi {
			hi = 1
		}
	}
	return numericChartScale(a, lo, hi)
}

func numericChartScale(a *model.ChartAxis, lo, hi float64) *ChartScaleDTO {
	if a.Minimum != nil {
		lo = *a.Minimum
	}
	if a.Maximum != nil {
		hi = *a.Maximum
	}
	if hi <= lo {
		if a.Maximum == nil {
			hi = lo + math.Max(1, math.Abs(lo)*.1)
		} else if a.Minimum == nil {
			lo = hi - math.Max(1, math.Abs(hi)*.1)
		}
	}
	if math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) || hi <= lo {
		return nil
	}
	// Automatic bounds leave room beyond the data, including exact tick multiples.
	raw := (hi - lo) / 6
	headroom := (hi - lo) * 0.05
	if a.Maximum == nil && hi > 0 {
		hi += headroom
	}
	if a.Minimum == nil && lo < 0 {
		lo -= headroom
	}
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	norm := raw / mag
	unit := 10.0
	for _, candidate := range []float64{1, 2, 2.5, 5, 10} {
		if norm <= candidate {
			unit = candidate
			break
		}
	}
	step := unit * mag
	if a.MajorUnit > 0 {
		step = a.MajorUnit
	}
	if math.IsInf(step, 0) || math.IsNaN(step) || step <= 0 {
		return nil
	}
	// Thin crowded ticks to multiples of the requested unit without losing bounds.
	if count := (hi - lo) / step; count > 199 {
		multiple := math.Ceil((hi - lo) / 199 / step)
		if math.IsInf(multiple, 0) {
			step = (hi - lo) / 199
		} else {
			step *= multiple
		}
	}
	if a.Minimum == nil {
		lo = math.Floor(lo/step) * step
	}
	if a.Maximum == nil {
		hi = math.Ceil(hi/step) * step
	}
	out := &ChartScaleDTO{Min: lo, Max: hi, Step: step, Labels: []string{}}
	for i := 0; i <= 200; i++ {
		v := lo + float64(i)*step
		if v > hi+step*1e-8 {
			break
		}
		out.Labels = append(out.Labels, numfmt.Format(v, a.Format))
	}
	return out
}

func chartScatterScale(c *model.Chart, data document.ChartData, horizontal bool) *ChartScaleDTO {
	if c.Type != "scatter" {
		return nil
	}
	axis := c.ValueAxis
	if horizontal {
		axis = c.CategoryAxis
	}
	if axis == nil {
		return nil
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, series := range data.Series {
		for i, y := range series.Values {
			if y == nil {
				continue
			}
			x := float64(i + 1)
			if len(series.X) > 0 {
				if i >= len(series.X) || series.X[i] == nil {
					continue
				}
				x = *series.X[i]
			}
			if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(*y) || math.IsInf(*y, 0) {
				continue
			}
			value := *y
			if horizontal {
				value = x
			}
			lo = math.Min(lo, value)
			hi = math.Max(hi, value)
		}
	}
	if math.IsInf(lo, 0) {
		lo, hi = 0, 1
	}
	if lo == hi {
		hi = lo + math.Max(1, math.Abs(lo)*.1)
	}
	return numericChartScale(axis, lo, hi)
}
