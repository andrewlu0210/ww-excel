package document

import (
	"math"
	"reflect"
	"testing"
	"ww-excel/internal/model"
)

func TestChartLabelComponentsAndPointOverrides(t *testing.T) {
	a, b := 30.0, 70.0
	text := "A&B\n備註"
	off := false
	labels := &model.ChartLabels{ShowValue: true, ShowPercent: true, ShowCategory: true, ShowSeries: true, Format: "0.0", PercentFormat: "0.0%", Separator: " / ", Points: map[int]model.ChartLabelPoint{1: {CustomText: &text}}}
	got := FormatChartLabels(labels, []*float64{&a, &b, nil}, "銷售", []string{"甲", "乙", "丙"})
	want := []string{"銷售 / 甲 / 30.0 / 30.0%", text, ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("labels=%q", got)
	}
	labels.Points[0] = model.ChartLabelPoint{ShowValue: &off, ShowPercent: &off}
	if got := FormatChartLabels(labels, []*float64{&a, &b}, "銷售", []string{"甲", "乙"})[0]; got != "銷售 / 甲" {
		t.Fatal(got)
	}
	labels.Points[0] = model.ChartLabelPoint{Hidden: true}
	if FormatChartLabels(labels, []*float64{&a}, "", nil)[0] != "" {
		t.Fatal("hidden point displayed")
	}
}

func TestMoveChartLabelPreservesPointAndChart(t *testing.T) {
	d := New()
	defer d.Close()
	sh := d.wb.Sheets[0]
	sh.Set(0, 0, model.NumberCell(10))
	sh.Set(1, 0, model.NumberCell(20))
	text := "Keep\nLabel"
	original := model.Object{ID: 3, Kind: model.ObjectChart, Anchor: model.Anchor{Row: 4, Col: 5, DX: 8, DY: 9}, W: 500, H: 300, Chart: &model.Chart{Type: "column", Series: []model.ChartSeries{{Values: "A1:A2", Labels: &model.ChartLabels{ShowValue: true, FontSize: 12, Points: map[int]model.ChartLabelPoint{0: {Center: true, CustomText: &text, Color: "FF0000", Layout: &model.ChartLabelLayout{X: .1, Y: .2, W: .3, H: .1, XMode: "factor", YMode: "factor"}}, 1: {Hidden: true}}}}}}}
	sh.Layout.Objects = []model.Object{original}
	before := sh.Layout.Clone()
	cmd, err := d.MoveChartLabelCommand(0, 3, 0, 0, .45, .35)
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	got := sh.Layout.Objects[0]
	if got.Anchor != original.Anchor || got.W != original.W || got.H != original.H {
		t.Fatal("drag moved the whole chart")
	}
	p := got.Chart.Series[0].Labels.Points[0]
	if p.Center || p.Color != "FF0000" || p.CustomText == nil || *p.CustomText != text || p.Layout.X != .45 || p.Layout.Y != .35 || p.Layout.W != .3 || p.Layout.H != .1 || p.Layout.XMode != "edge" {
		t.Fatalf("lost label settings: %+v", p)
	}
	if !got.Chart.Series[0].Labels.Points[1].Hidden {
		t.Fatal("changed another point")
	}
	d.Undo()
	if !reflect.DeepEqual(sh.Layout, before) {
		t.Fatal("undo lost original chart")
	}
	d.Redo()
	if sh.Layout.Objects[0].Chart.Series[0].Labels.Points[0].Layout.X != .45 {
		t.Fatal("redo lost position")
	}
}

func TestMoveChartLabelRejectsInvalidTargetsAndCoordinates(t *testing.T) {
	d := New()
	defer d.Close()
	sh := d.wb.Sheets[0]
	sh.Set(0, 0, model.NumberCell(1))
	sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, Chart: &model.Chart{Type: "column", Series: []model.ChartSeries{{Values: "A1", Labels: &model.ChartLabels{ShowValue: true, Points: map[int]model.ChartLabelPoint{0: {Hidden: true}}}}}}}}
	for _, tc := range []struct {
		sheet, id, series, point int
		x, y                     float64
	}{{-1, 1, 0, 0, .5, .5}, {0, 99, 0, 0, .5, .5}, {0, 1, -1, 0, .5, .5}, {0, 1, 0, 1, .5, .5}, {0, 1, 0, 0, math.NaN(), .5}, {0, 1, 0, 0, math.Inf(1), .5}, {0, 1, 0, 0, 3, .5}, {0, 1, 0, 0, .5, .5}} {
		if _, err := d.MoveChartLabelCommand(tc.sheet, tc.id, tc.series, tc.point, tc.x, tc.y); err == nil {
			t.Fatalf("invalid drag accepted: %+v", tc)
		}
	}
}

func TestMoveDefaultPieLabelKeepsPercentages(t *testing.T) {
	d := New()
	defer d.Close()
	sh := d.wb.Sheets[0]
	sh.Set(0, 0, model.NumberCell(30))
	sh.Set(1, 0, model.NumberCell(70))
	sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, W: 400, H: 300, Chart: &model.Chart{Type: "pie", Series: []model.ChartSeries{{Values: "A1:A2"}}}}}
	cmd, err := d.MoveChartLabelCommand(0, 1, 0, 1, .6, .4)
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	c := sh.Layout.Objects[0].Chart
	if got := FormatChartLabels(c.Series[0].Labels, []*float64{ptrFloat(30), ptrFloat(70)}, "", nil); !reflect.DeepEqual(got, []string{"30%", "70%"}) {
		t.Fatalf("lost default labels: %q", got)
	}
	d.Undo()
	if sh.Layout.Objects[0].Chart.Series[0].Labels != nil {
		t.Fatal("undo did not restore implicit pie labels")
	}
}
func ptrFloat(v float64) *float64 { return &v }
