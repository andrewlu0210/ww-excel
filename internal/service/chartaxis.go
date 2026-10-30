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
		for _, s := range data.Series {
			if i >= len(s.Values) || s.Values[i] == nil {
				continue
			}
			v := *s.Values[i]
			if math.IsNaN(v) || math.IsInf(v, 0) {
				continue
			}
			if c.Stacked && c.Type != "line" {
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
	if c.Type != "line" {
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
	if a.Minimum != nil {
		lo = *a.Minimum
	}
	if a.Maximum != nil {
		hi = *a.Maximum
	}
	if math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) || hi <= lo {
		return nil
	}
	raw := (hi - lo) / 6
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
	if math.IsInf(step, 0) || math.IsNaN(step) || step <= 0 || (hi-lo)/step > 200 {
		return nil
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
