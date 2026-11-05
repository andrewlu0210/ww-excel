package document

import (
	"math"
	"testing"
	"ww-excel/internal/model"
)

func TestChartAxisValidation(t *testing.T) {
	number := func(v float64) *float64 { return &v }
	for _, a := range []*model.ChartAxis{{Minimum: number(5), Maximum: number(5)}, {Minimum: number(10), Maximum: number(1)}, {Minimum: number(math.NaN())}, {Maximum: number(math.Inf(1))}, {MajorUnit: -1}, {MajorUnit: math.Inf(1)}, {FontSize: 500}, {LineColor: "red"}, {MajorTick: "invalid"}} {
		if validateChartAxes(&model.Chart{ValueAxis: a}) == nil {
			t.Fatalf("accepted invalid axis: %+v", a)
		}
	}
	if err := validateChartAxes(&model.Chart{ValueAxis: &model.ChartAxis{Minimum: number(-100), Maximum: number(100), MajorUnit: 25, FontSize: 12, LineColor: "123456", MajorTick: "cross"}}); err != nil {
		t.Fatal(err)
	}
}
