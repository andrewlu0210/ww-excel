package xlsxfile

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"github.com/xuri/excelize/v2"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func advancedChartFixture(t *testing.T) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	for r := 1; r <= 4; r++ {
		f.SetCellValue("Sheet1", string(rune('A'+0))+string(rune('0'+r)), r*10)
		f.SetCellValue("Sheet1", string(rune('B'+0))+string(rune('0'+r)), r*20)
	}
	if err := f.AddChart("Sheet1", "D2", &excelize.Chart{Type: excelize.Line, Title: excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: "Original", Font: &excelize.Font{Family: "Arial", Bold: true, Size: 18}}}}, Series: []excelize.ChartSeries{{Name: `"Sales"`, Categories: "Sheet1!$A$1:$A$3", Values: "Sheet1!$B$1:$B$3"}}, Legend: excelize.ChartLegend{Position: "r"}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := f.Pkg.Load("xl/charts/chart1.xml")
	data := string(raw.([]byte))
	data = strings.Replace(data, "</chartSpace>", `<c:extLst xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:ext uri="urn:keep"><x:custom xmlns:x="urn:custom" token="stay"/></c:ext></c:extLst></chartSpace>`, 1)
	// Keep an advanced per-series property not represented by the app model.
	data = strings.Replace(data, "</ser>", `<c:extLst xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:ext uri="urn:series"><x:custom xmlns:x="urn:custom" token="series-stay"/></c:ext></c:extLst></ser>`, 1)
	label := &model.ChartLabels{RawXML: `<c:dLbl><c:idx val="1"/><c:tx><c:rich><a:bodyPr/><a:p><a:r><a:t>Custom</a:t></a:r></a:p></c:rich></c:tx><c:extLst><c:ext uri="urn:point"><x:keep xmlns:x="urn:point"/></c:ext></c:extLst></c:dLbl><c:numFmt formatCode="0.0"/><c:txPr><a:bodyPr/><a:p><a:pPr><a:defRPr sz="1200"><a:latin typeface="Arial"/></a:defRPr></a:pPr></a:p></c:txPr><c:showVal val="1"/>`}
	data = string(restoreChartLabels([]byte(data), []model.ChartSeries{{Labels: label}}))
	f.Pkg.Store("xl/charts/chart1.xml", []byte(data))
	f.Pkg.Store("xl/charts/_rels/chart1.xml.rels", []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.microsoft.com/office/2011/relationships/chartStyle" Target="style1.xml"/></Relationships>`))
	f.Pkg.Store("xl/charts/style1.xml", []byte(`<cs:chartStyle xmlns:cs="http://schemas.microsoft.com/office/drawing/2012/chartStyle" id="10"/>`))
	entry := f.ContentTypes.Overrides[0]
	entry.PartName = "/xl/charts/style1.xml"
	entry.ContentType = "application/vnd.ms-office.chartstyle+xml"
	f.ContentTypes.Overrides = append(f.ContentTypes.Overrides, entry)
	file := filepath.Join(t.TempDir(), "advanced.xlsx")
	if err := f.SaveAs(file); err != nil {
		t.Fatal(err)
	}
	return file
}
func TestAdvancedChartPreservesUnknownXMLReferencesAndResources(t *testing.T) {
	wb, base, err := Read(advancedChartFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	sh := wb.Sheets[0]
	old := sh.Layout.Clone()
	c := sh.Layout.Objects[0].Chart
	if c.Source == nil || !CanPreserveChart(c) {
		t.Fatal("missing provenance")
	}
	c.Title = "New & Title"
	c.Series[0].Values = "Sheet1!$B$2:$B$4"
	c.Series[0].Labels.Position = "outEnd"
	c.Series[0].Labels.FontSize = 16
	text := "Edited\nA&B"
	point := c.Series[0].Labels.Points[1]
	point.CustomText = &text
	point.Layout = &model.ChartLabelLayout{X: .2, Y: .1, XMode: "edge", YMode: "edge"}
	c.Series[0].Labels.Points[1] = point
	if !CanPreserveChart(c) {
		t.Fatal("supported edit fell back to rebuild")
	}
	for i := 0; i < 2; i++ {
		output := filepath.Join(t.TempDir(), "edited.xlsx")
		saved, err := Save(output, wb, base, Changes{OldLayouts: map[*model.Sheet]model.Layout{sh: old}})
		if err != nil {
			t.Fatal(err)
		}
		base = saved
		assertAdvancedChart(t, output, text)
	}
	var native bytes.Buffer
	if err := wwx.Write(&native, wb); err != nil {
		t.Fatal(err)
	}
	back, err := wwx.Read(&native)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "from-native.xlsx")
	saved, err := Save(output, back, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	assertAdvancedChart(t, output, text)
	c.Type = "column"
	if CanPreserveChart(c) {
		t.Fatal("structural edit must keep compatibility warning")
	}
}
func assertAdvancedChart(t *testing.T, file, text string) {
	t.Helper()
	back, base, err := Read(file)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	c := back.Sheets[0].Layout.Objects[0].Chart
	if c.Title != "New & Title" || c.Series[0].Values != "Sheet1!$B$2:$B$4" {
		t.Fatalf("title/references: %+v", c)
	}
	l := c.Series[0].Labels
	if l == nil || l.Position != "outEnd" || l.FontSize != 16 || l.Points[1].CustomText == nil || *l.Points[1].CustomText != text || l.Points[1].Layout == nil {
		t.Fatalf("label settings: %+v", l)
	}
	if !strings.Contains(c.Source.XML, "series-stay") || !strings.Contains(c.Source.XML, `token="stay"`) || !strings.Contains(c.Source.XML, "urn:point") {
		t.Fatalf("lost advanced markup: %s", c.Source.XML)
	}
	if len(c.Source.Resources) < 2 {
		t.Fatal("lost chart style relationship")
	}
	z, err := zip.OpenReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	parts := map[string]*zip.File{}
	for _, p := range z.File {
		parts[p.Name] = p
	}
	for rel, resource := range c.Source.Resources {
		if !strings.HasSuffix(rel, ".rels") {
			continue
		}
		var doc struct {
			Items []struct {
				Target string `xml:"Target,attr"`
				Mode   string `xml:"TargetMode,attr"`
			} `xml:"Relationship"`
		}
		if err := xml.Unmarshal(resource.Data, &doc); err != nil {
			t.Fatal(err)
		}
		owner := strings.TrimSuffix(strings.Replace(rel, "/_rels/", "/", 1), ".rels")
		for _, r := range doc.Items {
			if r.Mode == "External" {
				continue
			}
			target := path.Clean(path.Join(path.Dir(owner), r.Target))
			if parts[target] == nil {
				t.Fatalf("missing relationship target %s", target)
			}
		}
	}
}
func TestChartLabelOverridesRoundTrip(t *testing.T) {
	custom := "A&B\n自訂"
	off := false
	l := &model.ChartLabels{Position: "t", ShowValue: true, ShowCategory: true, FontName: "Arial", FontSize: 15, Bold: true, Color: "123456", Fill: "FFFFFF", Border: "808080", Rotation: 20, Separator: " / ", Points: map[int]model.ChartLabelPoint{0: {CustomText: &custom, ShowPercent: &off, FontSize: 20, Color: "FF0000", Layout: &model.ChartLabelLayout{X: .3, Y: .2, XMode: "edge", YMode: "edge"}}}}
	raw := chartLabelsXML(l)
	var r rawChartLabels
	if err := xml.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	got := r.model(nil)
	if got.Position != "t" || !got.Bold || got.FontName != "Arial" || got.Color != "123456" || got.Rotation != 20 || got.Points[0].CustomText == nil || *got.Points[0].CustomText != custom || got.Points[0].Layout.X != .3 {
		t.Fatalf("labels=%+v", got)
	}
	got.Disabled = true
	raw = chartLabelsXML(got)
	if err := xml.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	if !r.model(nil).Disabled {
		t.Fatal("disabled labels reappeared")
	}
}

func TestChartAxisTitlesRoundTrip(t *testing.T) {
	for _, typ := range []string{"column", "bar", "scatter"} {
		t.Run(typ, func(t *testing.T) {
			f := excelize.NewFile()
			defer f.Close()
			chart := &model.Chart{Type: typ, Title: "Chart", XTitle: "Horizontal", YTitle: "Vertical", Legend: "none", Series: []model.ChartSeries{{Name: `"Sales"`, Categories: "Sheet1!$A$1:$A$3", Values: "Sheet1!$B$1:$B$3"}}}
			wb := model.NewWorkbook()
			sh := wb.Sheets[0]
			sh.Name = "Sheet1"
			sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, Anchor: model.Anchor{Row: 2, Col: 3}, W: 500, H: 300, Chart: chart}}
			filename := filepath.Join(t.TempDir(), "axes.xlsx")
			base, err := Save(filename, wb, nil, Changes{})
			if err != nil {
				t.Fatal(err)
			}
			defer base.Close()
			back, loaded, err := Read(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer loaded.Close()
			got := back.Sheets[0].Layout.Objects[0].Chart
			if got.XTitle != chart.XTitle || got.YTitle != chart.YTitle {
				t.Fatalf("titles lost: %+v", got)
			}
			old := back.Sheets[0].Layout.Clone()
			got.XTitle = "Changed X"
			got.YTitle = "Changed Y"
			output := filepath.Join(t.TempDir(), "changed.xlsx")
			saved, err := Save(output, back, loaded, Changes{OldLayouts: map[*model.Sheet]model.Layout{back.Sheets[0]: old}})
			if err != nil {
				t.Fatal(err)
			}
			defer saved.Close()
			edited, eb, err := Read(output)
			if err != nil {
				t.Fatal(err)
			}
			defer eb.Close()
			got = edited.Sheets[0].Layout.Objects[0].Chart
			if got.XTitle != "Changed X" || got.YTitle != "Changed Y" {
				t.Fatalf("edited titles lost: %+v", got)
			}
		})
	}
}

func TestStandaloneChartNodeKeepsSelfClosingNamespaceValid(t *testing.T) {
	data := []byte(`<c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:dLbls/></c:chart>`)
	root, err := drawingNodes(data)
	if err != nil {
		t.Fatal(err)
	}
	raw := standaloneChartNode(data, root.all("dLbls")[0])
	var value rawChartLabels
	if err := xml.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("invalid standalone label: %s: %v", raw, err)
	}
}
