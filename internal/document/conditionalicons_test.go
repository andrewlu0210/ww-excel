package document

import (
	"fmt"
	"path/filepath"
	"testing"
	"ww-excel/internal/model"
)

func TestFixedDataBarBoundsAndCustomIcons(t *testing.T) {
	d := New()
	enter(t, d, 0, 0, 0, "0.5")
	sh := d.Workbook().Sheets[0]
	sh.Layout.CondFormats = []model.CFRule{{Type: "data_bar", Ranges: []model.Range{{R1: 2}}, Colors: []string{"F1C94D"}, DataBar: &model.CFDataBar{Minimum: model.CFThreshold{Type: "num", Value: "0"}, Maximum: model.CFThreshold{Type: "num", Value: "1"}, Solid: true, BorderColor: "F1C94D"}}}
	result, _ := d.CondFormats(0, plain).At(0, 0, sh.Get(0, 0))
	if result.Bar != .5 || result.BarColor != "F1C94D" || !result.BarSolid || result.BarBorderColor != "F1C94D" {
		t.Fatalf("fixed bar bounds/style lost: %+v", result)
	}
	for row, v := range []string{"0", "0.5", "1"} {
		enter(t, d, 0, row, 0, v)
	}
	sh.Layout.CondFormats = []model.CFRule{{Type: "icon_set", Ranges: []model.Range{{R1: 2}}, IconSet: &model.CFIconSet{Thresholds: []model.CFThreshold{{Type: "percent", Value: "0"}, {Type: "num", Value: "0"}, {Type: "num", Value: "1"}}, Icons: []model.CFIcon{{Set: "NoIcons"}, {Set: "NoIcons"}, {Set: "4TrafficLights", Index: 0}}}}}
	e := d.CondFormats(0, plain)
	for row := 0; row < 3; row++ {
		r, ok := e.At(row, 0, sh.Get(row, 0))
		if !ok || (r.Icon != nil) != (row == 2) {
			t.Fatalf("row %d: %+v", row, r)
		}
		if r.Icon != nil && (r.Icon.Set != "4TrafficLights" || r.Icon.Index != 0) {
			t.Fatalf("wrong completion icon: %+v", r.Icon)
		}
	}
	cloned := sh.Layout.Clone()
	cloned.CondFormats[0].IconSet.Icons[2].Index = 3
	if sh.Layout.CondFormats[0].IconSet.Icons[2].Index != 0 {
		t.Fatal("undo snapshot shares icon slice")
	}
}

func TestCustomIconsAcrossNativeAndExcelCopies(t *testing.T) {
	d := New()
	for row, v := range []string{"0", "0.5", "1"} {
		enter(t, d, 0, row, 0, v)
	}
	d.Workbook().Sheets[0].Layout.CondFormats = []model.CFRule{{Type: "icon_set", Ranges: []model.Range{{R1: 2}}, IconSet: &model.CFIconSet{Set: "3TrafficLights1", Thresholds: []model.CFThreshold{{Type: "percent", Value: "0"}, {Type: "num", Value: "0"}, {Type: "num", Value: "1"}}, Icons: []model.CFIcon{{Set: "NoIcons"}, {Set: "NoIcons"}, {Set: "4TrafficLights", Index: 0}}}}}
	for _, ext := range []string{".wwx", ".xlsx", ".xlsx"} {
		path := filepath.Join(t.TempDir(), "custom-icons"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		sh := back.Workbook().Sheets[0]
		rules := sh.Layout.CondFormats
		if len(rules) != 1 || rules[0].IconSet == nil || len(rules[0].IconSet.Icons) != 3 || rules[0].IconSet.Icons[0].Set != "NoIcons" {
			t.Fatalf("custom icon slots lost in %s: %+v", ext, rules)
		}
		e := back.CondFormats(0, plain)
		first, _ := e.At(0, 0, sh.Get(0, 0))
		done, _ := e.At(2, 0, sh.Get(2, 0))
		if first.Icon != nil || done.Icon == nil || done.Icon.Set != "4TrafficLights" || done.Icon.Index != 0 {
			t.Fatalf("wrong saved icons in %s: %+v %+v", ext, first, done)
		}
	}
}

func TestHiddenDataBarValueAcrossExcelCopies(t *testing.T) {
	for _, solid := range []bool{false, true} {
		t.Run(fmt.Sprintf("solid=%v", solid), func(t *testing.T) {
			d := New()
			enter(t, d, 0, 0, 0, "10")
			enter(t, d, 0, 1, 0, "20")
			d.Workbook().Sheets[0].Layout.CondFormats = []model.CFRule{{Type: "data_bar", Ranges: []model.Range{{R1: 1}}, Colors: []string{"638EC6"}, DataBar: &model.CFDataBar{Minimum: model.CFThreshold{Type: "min"}, Maximum: model.CFThreshold{Type: "max"}, Solid: solid, HideValue: true}}}
			for copyIndex := 0; copyIndex < 2; copyIndex++ {
				path := filepath.Join(t.TempDir(), "hidden-bar.xlsx")
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
				if len(rules) != 1 || rules[0].DataBar == nil || !rules[0].DataBar.HideValue || rules[0].DataBar.Solid != solid {
					t.Fatalf("copy %d lost settings: %+v", copyIndex, rules)
				}
				result, ok := back.CondFormats(0, plain).At(1, 0, sh.Get(1, 0))
				if !ok || !result.HideValue {
					t.Fatalf("copy %d displays hidden number: %+v", copyIndex, result)
				}
				d = back
			}
		})
	}
}
