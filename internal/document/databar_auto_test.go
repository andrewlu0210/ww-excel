package document

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"ww-excel/internal/model"
)

func TestAutomaticDataBarBoundsUseZeroBaseline(t *testing.T) {
	for _, tc := range []struct {
		name         string
		values, want []float64
		lo, hi       float64
	}{
		{"positive", []float64{10, 20}, []float64{.5, 1}, 0, 20},
		{"negative", []float64{-20, -10}, []float64{0, .5}, -20, 0},
		{"mixed", []float64{-10, 0, 20}, []float64{0, 1.0 / 3, 1}, -10, 20},
		{"with-zero", []float64{0, 10}, []float64{0, 1}, 0, 10},
		{"same-positive", []float64{10, 10}, []float64{1, 1}, 0, 10},
		{"same-negative", []float64{-10, -10}, []float64{0, 0}, -10, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := New()
			defer d.Close()
			for row, v := range tc.values {
				enter(t, d, 0, row, 0, fmt.Sprint(v))
			}
			sh := d.Workbook().Sheets[0]
			sh.Layout.CondFormats = []model.CFRule{{Type: "data_bar", Ranges: []model.Range{{R1: len(tc.values) - 1}}, DataBar: &model.CFDataBar{Minimum: model.CFThreshold{Type: "autoMin"}, Maximum: model.CFThreshold{Type: "autoMax"}}}}
			e := d.CondFormats(0, plain)
			p := e.rules[0]
			for _, bound := range []struct {
				kind string
				want float64
			}{{"autoMin", tc.lo}, {"autoMax", tc.hi}, {"min", p.lo}, {"max", p.hi}} {
				got, ok := e.cfThreshold(p, model.CFThreshold{Type: bound.kind}, 0, 0)
				if !ok || got != bound.want {
					t.Errorf("%s=%v ok=%v want %v", bound.kind, got, ok, bound.want)
				}
			}
			for row, want := range tc.want {
				got, ok := e.At(row, 0, sh.Get(row, 0))
				if !ok || math.Abs(got.Bar-want) > 1e-9 {
					t.Errorf("row %d: bar=%v want %v", row, got.Bar, want)
				}
			}
		})
	}
}

func TestAutomaticDataBarBoundsAcrossCopies(t *testing.T) {
	d := New()
	defer d.Close()
	enter(t, d, 0, 0, 0, "10")
	enter(t, d, 0, 1, 0, "20")
	d.Workbook().Sheets[0].Layout.CondFormats = []model.CFRule{{Type: "data_bar", Ranges: []model.Range{{R1: 1}}, Colors: []string{"638EC6"}, DataBar: &model.CFDataBar{Minimum: model.CFThreshold{Type: "autoMin"}, Maximum: model.CFThreshold{Type: "autoMax"}, Solid: true, HideValue: true}}}
	for _, ext := range []string{".wwx", ".xlsx", ".xlsx"} {
		path := filepath.Join(t.TempDir(), "automatic-bar"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		defer back.Close()
		sh := back.Workbook().Sheets[0]
		bar, ok := back.CondFormats(0, plain).At(0, 0, sh.Get(0, 0))
		if !ok || bar.Bar != .5 || !bar.HideValue || !bar.BarSolid {
			t.Fatalf("%s: %+v", ext, bar)
		}
		d = back
	}
}

func TestAutomaticDataBarBoundsIgnoreEmptyRange(t *testing.T) {
	d := New()
	defer d.Close()
	d.Workbook().Sheets[0].Layout.CondFormats = []model.CFRule{{Type: "data_bar", Ranges: []model.Range{{R1: 1}}}}
	e := d.CondFormats(0, plain)
	for _, kind := range []string{"autoMin", "autoMax", "min", "max"} {
		if _, ok := e.cfThreshold(e.rules[0], model.CFThreshold{Type: kind}, 0, 0); ok {
			t.Fatalf("%s accepted empty statistics", kind)
		}
	}
}
