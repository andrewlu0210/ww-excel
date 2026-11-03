package xlsxfile

import (
	"archive/zip"
	"bytes"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func TestColumnLineComboRoundTrips(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	for r, row := range [][]any{{"Year", "Attendance", "Sales"}, {"2012", 20000, 5000}, {"2013", 15000, 11200}, {"2014", 4000, 30000}} {
		for c, value := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			if err := f.SetCellValue("Sheet1", cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	leftMax, rightMax := 25000.0, 35000.0
	primary := &excelize.Chart{Type: excelize.Col, Series: []excelize.ChartSeries{{Name: "Sheet1!$B$1", Categories: "Sheet1!$A$2:$A$4", Values: "Sheet1!$B$2:$B$4", Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"5B9BD5"}}}}, YAxis: excelize.ChartAxis{Maximum: &leftMax, MajorUnit: 5000, MajorGridLines: true, NumFmt: excelize.ChartNumFmt{CustomNumFmt: "General"}}}
	secondary := &excelize.Chart{Type: excelize.Line, Series: []excelize.ChartSeries{{Name: "Sheet1!$C$1", Categories: "Sheet1!$A$2:$A$4", Values: "Sheet1!$C$2:$C$4", Line: excelize.LineOptions{Width: 2.25, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"ED7D31"}}}, Marker: excelize.ChartMarker{Symbol: "none"}}}, YAxis: excelize.ChartAxis{Secondary: true, Maximum: &rightMax, MajorUnit: 5000, NumFmt: excelize.ChartNumFmt{CustomNumFmt: `"NT$"#,##0`}}}
	secondary.Series[0].Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"ED7D31"}}
	if err := f.AddChart("Sheet1", "E2", primary, secondary); err != nil {
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
	sh := wb.Sheets[0]
	chart := sh.Layout.Objects[0].Chart
	check := func(c *model.Chart) {
		t.Helper()
		if c == nil || c.Type != "combo" || len(c.Series) != 2 || c.Series[0].Type != "column" || c.Series[0].Secondary || c.Series[1].Type != "line" || !c.Series[1].Secondary {
			t.Fatalf("lost combo series: %+v", c)
		}
		if c.ValueAxis == nil || c.SecondaryAxis == nil || c.ValueAxis.Maximum == nil || *c.ValueAxis.Maximum != leftMax || c.SecondaryAxis.Maximum == nil || *c.SecondaryAxis.Maximum != rightMax || c.SecondaryAxis.Format != `"NT$"#,##0` {
			t.Fatalf("lost axis settings: %+v / %+v", c.ValueAxis, c.SecondaryAxis)
		}
		if c.Series[0].Color != "5B9BD5" || c.Series[1].Color != "ED7D31" || c.Series[1].Marker != "none" || c.Series[1].LineWidth != 3 {
			t.Fatalf("lost series appearance: %+v", c.Series)
		}
	}
	check(chart)
	chart.ValueAxis.LineHidden = true
	chart.ValueAxis.FontSize = 9
	chart.ValueAxis.FontName = "Arial"
	chart.SecondaryAxis.MajorTick = "out"
	chart.SecondaryAxis.GridLines = false
	chart.SecondaryCategoryAxis = &model.ChartAxis{Hidden: true, HideLabels: true}
	clone := sh.Layout.Clone()
	*clone.Objects[0].Chart.SecondaryAxis.Maximum = 999
	if *chart.SecondaryAxis.Maximum != rightMax {
		t.Fatal("secondary axis clone aliases original")
	}
	var native bytes.Buffer
	if err := wwx.Write(&native, wb); err != nil {
		t.Fatal(err)
	}
	back, err := wwx.Read(&native)
	if err != nil {
		t.Fatal(err)
	}
	check(back.Sheets[0].Layout.Objects[0].Chart)
	for _, inPlace := range []bool{false, true} {
		var src *excelize.File
		ch := Changes{}
		if inPlace {
			src = base
			old := sh.Layout.Clone()
			sh.Layout.Objects[0].W += 20
			ch.OldLayouts = map[*model.Sheet]model.Layout{sh: old}
		}
		out := filepath.Join(t.TempDir(), "combo.xlsx")
		saved, err := Save(out, wb, src, ch)
		if err != nil {
			t.Fatal(err)
		}
		if !inPlace {
			saved.Close()
		}
		got, b, err := Read(out)
		if err != nil {
			t.Fatal(err)
		}
		c := got.Sheets[0].Layout.Objects[0].Chart
		check(c)
		if !c.ValueAxis.LineHidden || c.ValueAxis.FontSize != 9 || c.ValueAxis.FontName != "Arial" || c.SecondaryAxis.MajorTick != "out" || c.SecondaryAxis.GridLines || c.SecondaryCategoryAxis == nil || !c.SecondaryCategoryAxis.Hidden || !c.SecondaryCategoryAxis.HideLabels {
			t.Fatalf("axis appearance changed: %+v / %+v / %+v", c.ValueAxis, c.SecondaryAxis, c.SecondaryCategoryAxis)
		}
		b.Close()
	}
}

func TestComboAxesAreMatchedByIDAndSeriesOrder(t *testing.T) {
	const source = `<chartSpace><chart><plotArea><barChart><barDir val="col"/><ser><idx val="1"/><val><numRef><f>Sheet1!B2:B3</f></numRef></val></ser><axId val="20"/><axId val="21"/></barChart><lineChart><ser><idx val="0"/><val><numRef><f>Sheet1!C2:C3</f></numRef></val></ser><axId val="10"/><axId val="11"/></lineChart><valAx><axId val="21"/><axPos val="r"/><numFmt formatCode="0%"/><crossAx val="20"/></valAx><valAx><axId val="11"/><axPos val="l"/><numFmt formatCode="General"/><crossAx val="10"/></valAx><catAx><axId val="20"/><delete val="1"/><crossAx val="21"/></catAx><catAx><axId val="10"/><delete val="0"/><crossAx val="11"/></catAx></plotArea></chart></chartSpace>`
	var data bytes.Buffer
	zw := zip.NewWriter(&data)
	part, err := zw.Create("xl/charts/chart1.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte(source)); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data.Bytes()), int64(data.Len()))
	if err != nil {
		t.Fatal(err)
	}
	c := readChart(map[string]*zip.File{zr.File[0].Name: zr.File[0]}, zr.File[0].Name)
	check := func(c *model.Chart) {
		t.Helper()
		if c == nil || c.Type != "combo" || c.ValueAxis.Format != "General" || c.SecondaryAxis.Format != "0%" || c.CategoryAxis.Hidden || !c.SecondaryCategoryAxis.Hidden || len(c.Series) != 2 || c.Series[0].Type != "line" || c.Series[0].Secondary || c.Series[1].Type != "column" || !c.Series[1].Secondary {
			t.Fatalf("axis/series mapping: %+v", c)
		}
	}
	check(c)
	wb := model.NewWorkbook()
	wb.Sheets[0].Name = "Sheet1"
	wb.Sheets[0].Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, W: 500, H: 300, Chart: c}}
	path := filepath.Join(t.TempDir(), "reverse.xlsx")
	saved, err := Save(path, wb, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	saved.Close()
	back, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	check(back.Sheets[0].Layout.Objects[0].Chart)
}
