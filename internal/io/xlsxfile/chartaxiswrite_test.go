package xlsxfile

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func TestChartAxisSettingsRoundTrip(t *testing.T) {
	for _, typ := range []string{"column", "bar", "line", "area", "scatter", "combo"} {
		t.Run(typ, func(t *testing.T) {
			wb := model.NewWorkbook()
			sh := wb.Sheets[0]
			sh.Name = "Data"
			for r := 0; r < 3; r++ {
				sh.Set(r, 0, model.NumberCell(float64(r+1)))
				sh.Set(r, 1, model.NumberCell(float64((r+1)*20)))
			}
			c := &model.Chart{Type: typ, Legend: "right", Series: []model.ChartSeries{{Categories: "Data!$A$1:$A$3", Values: "Data!$B$1:$B$3"}}}
			if typ == "combo" {
				c.Series[0].Type = "column"
				c.Series = append(c.Series, model.ChartSeries{Type: "line", Secondary: true, Categories: c.Series[0].Categories, Values: c.Series[0].Values})
			}
			sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, W: 500, H: 300, Chart: c}}
			source := filepath.Join(t.TempDir(), "source.xlsx")
			base, err := Save(source, wb, nil, Changes{})
			if err != nil {
				t.Fatal(err)
			}
			base.Close()
			wb, base, err = Read(source)
			if err != nil {
				t.Fatal(err)
			}
			defer base.Close()
			sh = wb.Sheets[0]
			old := sh.Layout.Clone()
			c = sh.Layout.Objects[0].Chart
			min, max := -100., 400.
			c.ValueAxis = &model.ChartAxis{Minimum: &min, Maximum: &max, MajorUnit: 50, Format: `"NT$"#,##0`, FontName: "Arial", FontSize: 14, GridLines: true, MajorTick: "cross", LineColor: "FF0000", HideLabels: true}
			c.CategoryAxis = &model.ChartAxis{Format: "0.0", FontName: "Arial", FontSize: 12, GridLines: true, MajorTick: "in", LineHidden: true}
			if typ == "scatter" {
				x0, x1 := 0., 10.
				c.CategoryAxis.Minimum = &x0
				c.CategoryAxis.Maximum = &x1
				c.CategoryAxis.MajorUnit = 2
			}
			if typ == "combo" {
				lo, hi := 0., 1000.
				c.SecondaryAxis = &model.ChartAxis{Minimum: &lo, Maximum: &hi, MajorUnit: 250, Format: "0.0", FontName: "Arial", FontSize: 11, MajorTick: "out", LineColor: "00AA00"}
			}
			if !CanPreserveChart(c) {
				t.Fatal("axis edit must preserve imported markup")
			}
			output := filepath.Join(t.TempDir(), "edited.xlsx")
			saved, err := Save(output, wb, base, Changes{OldLayouts: map[*model.Sheet]model.Layout{sh: old}})
			if err != nil {
				t.Fatal(err)
			}
			defer saved.Close()
			var native bytes.Buffer
			if err := wwx.Write(&native, wb); err != nil {
				t.Fatal(err)
			}
			back, err := wwx.Read(&native)
			if err != nil {
				t.Fatal(err)
			}
			nativeOutput := filepath.Join(t.TempDir(), "native.xlsx")
			nb, err := Save(nativeOutput, back, nil, Changes{})
			if err != nil {
				t.Fatal(err)
			}
			defer nb.Close()
			for _, filename := range []string{output, nativeOutput} {
				reopened, loaded, err := Read(filename)
				if err != nil {
					t.Fatal(err)
				}
				got := reopened.Sheets[0].Layout.Objects[0].Chart
				assertValueAxisStructure(t, []byte(got.Source.XML))
				defer loaded.Close()
				a := got.ValueAxis
				if a.Minimum == nil || *a.Minimum != -100 || a.Maximum == nil || *a.Maximum != 400 || a.MajorUnit != 50 || a.FontSize != 14 || a.FontName != "Arial" || a.Format != c.ValueAxis.Format || !a.GridLines || a.MajorTick != "cross" || a.LineColor != "FF0000" || !a.HideLabels {
					t.Fatalf("lost %s axis: %+v", typ, a)
				}
				if got.CategoryAxis.FontSize != 12 || !got.CategoryAxis.GridLines || !got.CategoryAxis.LineHidden {
					t.Fatalf("lost category style: %+v", got.CategoryAxis)
				}
				if typ == "scatter" && (got.CategoryAxis.Maximum == nil || *got.CategoryAxis.Maximum != 10 || got.CategoryAxis.MajorUnit != 2) {
					t.Fatal("mixed scatter X and Y axes")
				}
				if typ == "combo" && (got.SecondaryAxis.Maximum == nil || *got.SecondaryAxis.Maximum != 1000 || got.SecondaryAxis.LineColor != "00AA00") {
					t.Fatal("lost independent right axis")
				}
				if filename == output {
					resetOld := reopened.Sheets[0].Layout.Clone()
					got.ValueAxis.Minimum = nil
					got.ValueAxis.Maximum = nil
					got.ValueAxis.MajorUnit = 0
					resetPath := filepath.Join(t.TempDir(), "reset.xlsx")
					rb, err := Save(resetPath, reopened, loaded, Changes{OldLayouts: map[*model.Sheet]model.Layout{reopened.Sheets[0]: resetOld}})
					if err != nil {
						t.Fatal(err)
					}
					defer rb.Close()
					reset, zb, err := Read(resetPath)
					if err != nil {
						t.Fatal(err)
					}
					defer zb.Close()
					ra := reset.Sheets[0].Layout.Objects[0].Chart.ValueAxis
					if ra.Minimum != nil || ra.Maximum != nil || ra.MajorUnit != 0 || ra.FontSize != 14 || ra.Format != c.ValueAxis.Format {
						t.Fatalf("reset lost settings: %+v", ra)
					}
				}
			}
		})
	}
}
func TestAxisEditPreservesUnknownXML(t *testing.T) {
	wb, base, err := Read(advancedChartFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	c := wb.Sheets[0].Layout.Objects[0].Chart
	c.Source.XML = strings.Replace(c.Source.XML, "</valAx>", `<c:extLst xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:ext uri="axis-keep"><x:axis xmlns:x="urn:axis"/></c:ext></c:extLst></valAx>`, 1)
	prior := wb.Sheets[0].Layout.Clone()
	c.ValueAxis.FontSize = 18
	c.ValueAxis.FontName = "Arial"
	c.ValueAxis.LineColor = "112233"
	c.ValueAxis.LineHidden = false
	out := filepath.Join(t.TempDir(), "axes.xlsx")
	saved, err := Save(out, wb, base, Changes{OldLayouts: map[*model.Sheet]model.Layout{wb.Sheets[0]: prior}})
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	back, b, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	raw := back.Sheets[0].Layout.Objects[0].Chart.Source.XML
	for _, token := range []string{"axis-keep", "series-stay", `token="stay"`, "urn:point"} {
		if !strings.Contains(raw, token) {
			t.Fatalf("lost untouched markup %s", token)
		}
	}
}
