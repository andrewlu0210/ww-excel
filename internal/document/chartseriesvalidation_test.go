package document

import (
	"math"
	"testing"
	"ww-excel/internal/model"
)

func TestSeriesStyleValidation(t *testing.T) {
	for _, s := range []model.ChartSeries{{Color: "bad"}, {LineWidth: -1}, {LineWidth: 31}, {LineWidth: math.NaN()}, {Marker: "bogus"}} {
		if validateChartSeriesStyle(&model.Chart{Series: []model.ChartSeries{s}}) == nil {
			t.Fatalf("accepted %+v", s)
		}
	}
	for _, gap := range []float64{-1, 501, .5, math.NaN(), math.Inf(1)} {
		if validateChartSeriesStyle(&model.Chart{GapWidth: &gap}) == nil {
			t.Fatal("invalid gap")
		}
	}
	for _, size := range []int{-1, 1, 91} {
		if validateChartSeriesStyle(&model.Chart{HoleSize: size}) == nil {
			t.Fatal("invalid hole size")
		}
	}
	if err := validateChartSeriesStyle(&model.Chart{Series: []model.ChartSeries{{Color: "AA00FF", LineWidth: 2.25, Marker: "star"}}}); err != nil {
		t.Fatal(err)
	}
}
