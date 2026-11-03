package document

import (
	"math"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"ww-excel/internal/model"
)

var iconPresets = []string{"3Arrows", "3ArrowsGray", "3Flags", "3TrafficLights1", "3TrafficLights2", "3Signs", "3Symbols", "3Symbols2", "4Arrows", "4ArrowsGray", "4RedToBlack", "4Rating", "4TrafficLights", "5Arrows", "5ArrowsGray", "5Rating", "5Quarters", "3Stars", "3Triangles", "5Boxes"}

func TestIconPresetStatesAndRoundTrips(t *testing.T) {
	d := New()
	for row, set := range iconPresets {
		n := model.IconSetSize(set)
		for col := 0; col < n; col++ {
			enter(t, d, 0, row, col, fmtFloat(float64(col)*100/float64(n-1)))
		}
		rule := model.CFRule{Type: "icon_set", IconSet: &model.CFIconSet{Set: set, Reverse: row%2 == 1, HideValue: row%3 == 0}}
		must(t, d)(d.AddCFRuleCommand(0, []model.Range{{R0: row, R1: row, C1: n - 1}}, rule))
	}
	verify := func(doc *Document) {
		t.Helper()
		sh := doc.Workbook().Sheets[0]
		e := doc.CondFormats(0, plain)
		for row, set := range iconPresets {
			n := model.IconSetSize(set)
			for col := 0; col < n; col++ {
				result, ok := e.At(row, col, sh.Get(row, col))
				want := col
				if row%2 == 1 {
					want = n - 1 - col
				}
				if !ok || result.Icon == nil || result.Icon.Set != set || result.Icon.Index != want || result.HideValue != (row%3 == 0) {
					t.Fatalf("%s row %d col %d: %+v", set, row, col, result)
				}
			}
		}
	}
	verify(d)
	for _, ext := range []string{".wwx", ".xlsx", ".xlsx", ".wwx"} {
		path := filepath.Join(t.TempDir(), "icons"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		verify(back)
		d.Close()
		d = back
	}
	d.Close()
}

func fmtFloat(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

func TestIconThresholdBoundariesCustomSlotsAndUndo(t *testing.T) {
	d := New()
	for row, v := range []string{"20", "50", "90"} {
		enter(t, d, 0, row, 0, v)
	}
	rule := model.CFRule{Type: "icon_set", IconSet: &model.CFIconSet{Set: "3Arrows", HideValue: true, Thresholds: []model.CFThreshold{{Type: "num", Value: "0"}, {Type: "num", Value: "50"}, {Type: "num", Value: "90", Strict: true}}}}
	must(t, d)(d.AddCFRuleCommand(0, []model.Range{{R1: 2}}, rule))
	states := func() []int {
		e := d.CondFormats(0, plain)
		var states []int
		for r := 0; r < 3; r++ {
			v, _ := e.At(r, 0, d.Workbook().Sheets[0].Get(r, 0))
			if v.Icon == nil {
				states = append(states, -1)
			} else {
				states = append(states, v.Icon.Index)
			}
		}
		return states
	}
	if got := states(); !reflect.DeepEqual(got, []int{0, 1, 1}) {
		t.Fatalf("strict boundary: %v", got)
	}
	enter(t, d, 0, 2, 0, "91")
	if states()[2] != 2 {
		t.Fatal("edited value did not update icon")
	}
	d.Undo()
	if states()[2] != 1 {
		t.Fatal("undo did not restore icon")
	}
	d.Redo()
	if states()[2] != 2 {
		t.Fatal("redo did not restore icon")
	}
	custom := *rule.IconSet
	custom.Reverse = true
	custom.Icons = []model.CFIcon{{Set: "3Flags", Index: 0}, {Set: "NoIcons"}, {Set: "5Rating", Index: 4}}
	must(t, d)(d.AddCFRuleCommand(0, []model.Range{{R1: 2}}, model.CFRule{Type: "icon_set", IconSet: &custom}))
	e := d.CondFormats(0, plain)
	for row, want := range []*model.CFIcon{{Set: "5Rating", Index: 4}, nil, {Set: "3Flags", Index: 0}} {
		got, ok := e.At(row, 0, d.Workbook().Sheets[0].Get(row, 0))
		if !ok || !reflect.DeepEqual(got.Icon, want) || !got.HideValue {
			t.Fatalf("custom/reverse row %d: %+v", row, got)
		}
	}
	for _, ext := range []string{".xlsx", ".wwx"} {
		path := filepath.Join(t.TempDir(), "custom"+ext)
		if err := d.SaveAs(path); err != nil {
			t.Fatal(err)
		}
		back, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		sh := back.Workbook().Sheets[0]
		result, _ := back.CondFormats(0, plain).At(1, 0, sh.Get(1, 0))
		if result.Icon != nil || !result.HideValue || !reflect.DeepEqual(sh.Layout.CondFormats[0].IconSet.Icons, custom.Icons) || !reflect.DeepEqual(sh.Layout.CondFormats[0].IconSet.Thresholds, custom.Thresholds) {
			t.Fatalf("custom slots changed in %s", ext)
		}
		back.Close()
	}
}

func TestIconPercentPercentileAndFormula(t *testing.T) {
	for _, kind := range []string{"percent", "percentile", "formula"} {
		t.Run(kind, func(t *testing.T) {
			d := New()
			for row, v := range []string{"0", "20", "50", "90", "100"} {
				enter(t, d, 0, row, 0, v)
			}
			thresholds := []model.CFThreshold{{Type: "num", Value: "0"}, {Type: kind, Value: "40"}, {Type: kind, Value: "80"}}
			if kind == "formula" {
				enter(t, d, 0, 0, 1, "40")
				enter(t, d, 0, 1, 1, "80")
				thresholds[1].Value = "$B$1"
				thresholds[2].Value = "$B$2"
			}
			must(t, d)(d.AddCFRuleCommand(0, []model.Range{{R1: 4}}, model.CFRule{Type: "icon_set", IconSet: &model.CFIconSet{Set: "3Arrows", Thresholds: thresholds}}))
			e := d.CondFormats(0, plain)
			wants := []int{0, 0, 1, 2, 2}
			if kind == "percentile" {
				wants = []int{0, 0, 1, 1, 2}
			}
			for row, want := range wants {
				got, _ := e.At(row, 0, d.Workbook().Sheets[0].Get(row, 0))
				if got.Icon == nil || got.Icon.Index != want {
					t.Fatalf("row %d: %+v", row, got)
				}
			}
		})
	}
}

func TestIconValidationAndNonNumericValues(t *testing.T) {
	d := New()
	for _, set := range []*model.CFIconSet{nil, {Set: "Future"}, {Set: "3Arrows", Thresholds: []model.CFThreshold{{Type: "num", Value: "0"}}}, {Set: "3Arrows", Thresholds: []model.CFThreshold{{Type: "num", Value: "0"}, {Type: "num", Value: "NaN"}, {Type: "num", Value: "1"}}}} {
		if _, err := d.AddCFRuleCommand(0, []model.Range{{R1: 1}}, model.CFRule{Type: "icon_set", IconSet: set}); err == nil {
			t.Fatalf("invalid set accepted: %+v", set)
		}
	}
	sh := d.Workbook().Sheets[0]
	sh.Layout.CondFormats = []model.CFRule{{Type: "icon_set", Ranges: []model.Range{{R1: 5}}, IconSet: &model.CFIconSet{Set: "3Arrows", Thresholds: model.IconThresholds(3)}}}
	e := d.CondFormats(0, plain)
	for _, cell := range []model.Cell{{}, model.StringCell("text"), model.NumberCell(math.NaN()), model.NumberCell(math.Inf(1))} {
		result, _ := e.At(0, 0, cell)
		if result.Icon != nil || result.HideValue {
			t.Fatalf("invalid cell got icon: %+v", result)
		}
	}
}
