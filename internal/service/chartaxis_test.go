package service

import (
	"testing"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func TestImportedCurrencyAxis(t *testing.T) {
	a, b := 112500.0, 70080.0
	c := &model.Chart{Type: "column", ValueAxis: &model.ChartAxis{Format: `"NT$"#,##0`}}
	data := document.ChartData{Series: []document.SeriesData{{Values: []*float64{&a}}, {Values: []*float64{&b}}}}
	s := chartValueScale(c, data)
	if s == nil || s.Min != 0 || s.Max != 120000 || s.Step != 20000 || len(s.Labels) != 7 || s.Labels[0] != "NT$0" || s.Labels[6] != "NT$120,000" {
		t.Fatalf("unexpected currency axis: %+v", s)
	}
	min, max := 10000.0, 130000.0
	c.ValueAxis.Minimum = &min
	c.ValueAxis.Maximum = &max
	c.ValueAxis.MajorUnit = 30000
	s = chartValueScale(c, data)
	if s.Min != 10000 || s.Max != 130000 || s.Step != 30000 || s.Labels[1] != "NT$40,000" {
		t.Fatalf("lost explicit bounds: %+v", s)
	}
}

func TestImportedAxisLeavesHeadroomAtExactMaximum(t *testing.T) {
	for _, negative := range []bool{false, true} {
		values := []float64{20000, 15000, 4000, 18000, 10000, 8000}
		series := document.SeriesData{}
		for _, value := range values {
			if negative {
				value = -value
			}
			series.Values = append(series.Values, &value)
		}
		c := &model.Chart{Type: "column", ValueAxis: &model.ChartAxis{Format: "General"}}
		data := document.ChartData{Series: []document.SeriesData{series}}
		scale := chartValueScale(c, data)
		wantMin, wantMax := 0.0, 25000.0
		if negative {
			wantMin, wantMax = -25000, 0
		}
		if scale == nil || scale.Min != wantMin || scale.Max != wantMax || scale.Step != 5000 {
			t.Fatalf("automatic scale: %+v", scale)
		}
		c.ValueAxis.Minimum, c.ValueAxis.Maximum = &wantMin, &wantMax
		if !negative {
			*c.ValueAxis.Maximum = 20000
		} else {
			*c.ValueAxis.Minimum = -20000
		}
		scale = chartValueScale(c, data)
		if scale == nil || scale.Min != *c.ValueAxis.Minimum || scale.Max != *c.ValueAxis.Maximum {
			t.Fatalf("explicit bounds changed: %+v", scale)
		}
	}
}

func TestComboScalesUseOnlyTheirOwnSeries(t *testing.T) {
	left, right := 20000.0, 30000.0
	c := &model.Chart{Type: "combo", ValueAxis: &model.ChartAxis{Format: "General"}, SecondaryAxis: &model.ChartAxis{Format: `"NT$"#,##0`}, Series: []model.ChartSeries{{Type: "column"}, {Type: "line", Secondary: true}}}
	data := document.ChartData{Series: []document.SeriesData{{Values: []*float64{&left}}, {Values: []*float64{&right}}}}
	a, b := chartValueScale(c, data), chartSecondaryScale(c, data)
	if a == nil || a.Max != 25000 || b == nil || b.Max != 35000 || b.Step != 5000 || b.Labels[7] != "NT$35,000" {
		t.Fatalf("independent scales: %+v / %+v", a, b)
	}
	right = 900000
	if got := chartValueScale(c, data); got.Max != 25000 {
		t.Fatalf("right values changed left axis: %+v", got)
	}
	max := 1000000.0
	c.SecondaryAxis.Maximum = &max
	if got := chartSecondaryScale(c, data); got.Max != max {
		t.Fatalf("lost fixed right bound: %+v", got)
	}
	// A primary-axis line must contribute to the primary range.
	c.Series[1].Secondary = false
	if chartSecondaryScale(c, data) != nil || chartValueScale(c, data).Max <= 900000 {
		t.Fatal("series axis reassignment was ignored")
	}
}
