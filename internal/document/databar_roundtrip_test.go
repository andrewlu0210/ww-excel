package document

import (
	"fmt"
	"path/filepath"
	"testing"
	"ww-excel/internal/model"
)

func TestFixedDataBarBoundsAcrossCopies(t *testing.T) {
	for _, visual := range []struct {
		name   string
		solid  bool
		border string
	}{{"solid", true, ""}, {"border", false, "638EC6"}, {"solid-border", true, "638EC6"}} {
		for _, hidden := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/hidden=%v", visual.name, hidden), func(t *testing.T) {
				d := New()
				defer d.Close()
				enter(t, d, 0, 0, 0, "25")
				enter(t, d, 0, 1, 0, "50")
				want := model.CFDataBar{Minimum: model.CFThreshold{Type: "num", Value: "0"}, Maximum: model.CFThreshold{Type: "num", Value: "100"}, Solid: visual.solid, BorderColor: visual.border, HideValue: hidden}
				d.Workbook().Sheets[0].Layout.CondFormats = []model.CFRule{{Type: "data_bar", Ranges: []model.Range{{R1: 1}}, Colors: []string{"638EC6"}, DataBar: &want}}
				for _, ext := range []string{".xlsx", ".xlsx", ".wwx", ".xlsx"} {
					path := filepath.Join(t.TempDir(), "fixed-bar"+ext)
					if err := d.SaveAs(path); err != nil {
						t.Fatal(err)
					}
					back, err := Load(path)
					if err != nil {
						t.Fatal(err)
					}
					defer back.Close()
					sh := back.Workbook().Sheets[0]
					rules := sh.Layout.CondFormats
					if len(rules) != 1 {
						t.Fatalf("%s: expected one rule, got %d", ext, len(rules))
					}
					if rules[0].DataBar == nil || *rules[0].DataBar != want {
						t.Fatalf("%s lost bar bounds/settings: %+v", ext, rules[0].DataBar)
					}
					result, ok := back.CondFormats(0, plain).At(0, 0, sh.Get(0, 0))
					if !ok || result.Bar != .25 || result.HideValue != hidden {
						t.Fatalf("%s wrong bar display: %+v", ext, result)
					}
					d = back
				}
			})
		}
	}
}
