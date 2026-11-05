package xlsxfile

import (
	"bytes"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func TestSeriesAppearancePreservesImportedChartAndNativeRoundTrip(t *testing.T) {
	for _, typ := range []string{"column", "bar", "line", "area", "scatter", "combo", "doughnut"} {
		t.Run(typ, func(t *testing.T) {
			wb := model.NewWorkbook()
			sh := wb.Sheets[0]
			for r := 0; r < 3; r++ {
				sh.Set(r, 0, model.NumberCell(float64(r+1)))
				sh.Set(r, 1, model.NumberCell(float64((r+1)*20)))
			}
			c := &model.Chart{Type: typ, Legend: "none", Series: []model.ChartSeries{{Values: "B1:B3", Categories: "A1:A3"}}}
			if typ == "combo" {
				c.Series[0].Type = "column"
				c.Series = append(c.Series, model.ChartSeries{Type: "line", Secondary: true, Values: "B1:B3", Categories: "A1:A3"})
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
			previous := sh.Layout.Clone()
			c = sh.Layout.Objects[0].Chart
			if typ == "scatter" && !strings.Contains(c.Source.XML, `val="marker"`) {
				t.Fatal("new scatter chart must use markers without connecting lines")
			}
			for i := range c.Series {
				c.Series[i].Color = "A100CC"
				if typ == "line" || typ == "scatter" || c.Series[i].Type == "line" {
					c.Series[i].Marker = "diamond"
					c.Series[i].LineWidth = 4.25
				}
			}
			gap := 75.
			if typ == "column" || typ == "bar" || typ == "combo" {
				c.GapWidth = &gap
			}
			if typ == "doughnut" {
				c.HoleSize = 70
			}
			if !CanPreserveChart(c) {
				t.Fatal("style edit must preserve source XML")
			}
			out := filepath.Join(t.TempDir(), "style.xlsx")
			saved, err := Save(out, wb, base, Changes{OldLayouts: map[*model.Sheet]model.Layout{sh: previous}})
			if err != nil {
				t.Fatal(err)
			}
			defer saved.Close()
			var native bytes.Buffer
			if err := wwx.Write(&native, wb); err != nil {
				t.Fatal(err)
			}
			nw, err := wwx.Read(&native)
			if err != nil {
				t.Fatal(err)
			}
			nativeFile := filepath.Join(t.TempDir(), "native.xlsx")
			nb, err := Save(nativeFile, nw, nil, Changes{})
			if err != nil {
				t.Fatal(err)
			}
			defer nb.Close()
			for _, filename := range []string{out, nativeFile} {
				back, b, err := Read(filename)
				if err != nil {
					t.Fatal(err)
				}
				got := back.Sheets[0].Layout.Objects[0].Chart
				b.Close()
				for _, s := range got.Series {
					if s.Color != "A100CC" {
						t.Fatalf("lost series color: %+v", s)
					}
					if typ == "line" || typ == "scatter" || s.Type == "line" {
						if s.Marker != "diamond" || math.Abs(s.LineWidth-4.25) > 1e-4 {
							t.Fatalf("lost line style: %+v", s)
						}
					}
				}
				if c.GapWidth != nil && (got.GapWidth == nil || *got.GapWidth != 75) {
					t.Fatal("lost gap")
				}
				if typ == "doughnut" && got.HoleSize != 70 {
					t.Fatal("lost hole size")
				}
			}
		})
	}
}
func TestSeriesStyleKeepsUnknownExtensions(t *testing.T) {
	wb, base, err := Read(advancedChartFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	sh := wb.Sheets[0]
	old := sh.Layout.Clone()
	c := sh.Layout.Objects[0].Chart
	c.Series[0].Color = "112233"
	c.Series[0].LineWidth = 4
	c.Series[0].Marker = "star"
	out := filepath.Join(t.TempDir(), "styled.xlsx")
	saved, err := Save(out, wb, base, Changes{OldLayouts: map[*model.Sheet]model.Layout{sh: old}})
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
	for _, token := range []string{"urn:point", "series-stay", `token="stay"`} {
		if !strings.Contains(raw, token) {
			t.Fatalf("lost %s", token)
		}
	}
}

func TestSeriesMatchDisambiguatesSharedValuesByName(t *testing.T) {
	data := []byte(`<chart xmlns="http://schemas.openxmlformats.org/drawingml/2006/chart"><lineChart><ser><tx><strRef><f>"Primary"</f></strRef></tx><val><numRef><f>A1:A3</f></numRef></val></ser></lineChart><lineChart><ser><tx><strRef><f>"Renamed secondary"</f></strRef></tx><val><numRef><f>A1:A3</f></numRef></val></ser></lineChart></chart>`)
	root, err := drawingNodes(data)
	if err != nil {
		t.Fatal(err)
	}
	nodes := root.all("ser")
	got := matchChartSeries(nodes, data, model.ChartSeries{Type: "line", Name: `"Renamed secondary"`, Values: "A1:A3"}, map[*drawingNode]bool{})
	if got != nodes[1] {
		t.Fatal("same-value series received another series' name")
	}
}
