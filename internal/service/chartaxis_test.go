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
