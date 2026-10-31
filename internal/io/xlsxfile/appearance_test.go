package xlsxfile

import (
	"encoding/xml"
	"github.com/xuri/excelize/v2"
	"math"
	"path/filepath"
	"testing"
	"ww-excel/internal/model"
)

func TestChartAppearanceRoundTrip(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	f.SetCellValue("Sheet1", "A1", .4512)
	f.SetCellValue("Sheet1", "A2", .5488)
	if err := f.AddChart("Sheet1", "C1", &excelize.Chart{Type: excelize.Pie, Series: []excelize.ChartSeries{{Values: "Sheet1!$A$1:$A$2"}}}); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "source.xlsx")
	if err := f.SaveAs(source); err != nil {
		t.Fatal(err)
	}
	wb, base, err := Read(source)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	c := wb.Sheets[0].Layout.Objects[0].Chart
	c.PlotArea = &model.ChartPlotArea{X: .09, Y: .046, W: .8355, H: .89}
	c.Series[0].PointColors = map[int]string{0: "91BD30", 1: "000000"}
	c.Series[0].PointStyles = map[int]model.ChartPointStyle{0: {LineHidden: true, Gradient: &model.ChartGradient{Angle: 90, Stops: []model.ChartGradientStop{{Position: 0, Color: "A5C55E"}, {Position: 1, Color: "709621"}}}, Shadow: &model.ChartShadow{Color: "000000", Opacity: .63, Blur: 6, Y: 2}}}
	wb.Sheets[0].Layout.Zoom = 125
	path := filepath.Join(t.TempDir(), "roundtrip.xlsx")
	saved, err := Save(path, wb, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	saved.Close()
	back, b, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if len(back.Sheets[0].Layout.Objects) == 0 {
		t.Fatal("chart missing")
	}
	got := back.Sheets[0].Layout.Objects[0].Chart
	if got.PlotArea == nil || *got.PlotArea != *c.PlotArea {
		t.Fatalf("plot area: %+v", got.PlotArea)
	}
	point := got.Series[0].PointStyles[0]
	if point.Gradient == nil || len(point.Gradient.Stops) != 2 || point.Gradient.Stops[0].Color != "A5C55E" || point.Gradient.Angle != 90 || !point.LineHidden {
		t.Fatalf("point: %+v", point)
	}
	if point.Shadow == nil || point.Shadow.Blur != 6 || point.Shadow.Opacity != .63 || math.Abs(point.Shadow.Y-2) > 1e-6 {
		t.Fatalf("shadow: %+v", point.Shadow)
	}
	if back.Sheets[0].Layout.Zoom != 125 {
		t.Fatal("zoom lost")
	}
	cloneLayout := wb.Sheets[0].Layout.Clone()
	clone := cloneLayout.Objects[0].Chart
	clone.Series[0].PointStyles[0].Gradient.Stops[0].Color = "FFFFFF"
	clone.PlotArea.W = 1
	if c.Series[0].PointStyles[0].Gradient.Stops[0].Color != "A5C55E" || c.PlotArea.W == 1 {
		t.Fatal("chart clone aliases appearance")
	}
}
func TestDrawingGradientColorAndShadow(t *testing.T) {
	var p rawChartAppearance
	err := xml.Unmarshal([]byte(`<spPr><gradFill><gsLst><gs pos="0"><schemeClr val="accent1"><tint val="94000"/></schemeClr></gs><gs pos="100000"><schemeClr val="accent1"><shade val="78000"/></schemeClr></gs></gsLst><lin ang="5400000"/></gradFill><effectLst><outerShdw blurRad="57150" dist="19050" dir="5400000"><srgbClr val="000000"><alpha val="63000"/></srgbClr></outerShdw></effectLst></spPr>`), &p)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := p.model(map[string]string{"accent1": "91BD30"})
	if !ok || got.Gradient == nil || got.Gradient.Stops[0].Color != "9BC254" || got.Gradient.Stops[1].Color != "81A92A" {
		t.Fatalf("gradient: %+v", got.Gradient)
	}
	if got.Shadow == nil || got.Shadow.Opacity != .63 {
		t.Fatalf("shadow: %+v", got.Shadow)
	}
}
