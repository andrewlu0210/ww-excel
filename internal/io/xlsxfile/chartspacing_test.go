package xlsxfile

import (
	"archive/zip"
	"bytes"
	"github.com/xuri/excelize/v2"
	"path/filepath"
	"testing"
	"ww-excel/internal/model"
)

func TestImportedDateAxisAndColumnSpacing(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	part, err := zw.Create("xl/charts/chart1.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = part.Write([]byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><c:txPr><a:p><a:pPr><a:defRPr><a:latin typeface="Microsoft JhengHei UI"/></a:defRPr></a:pPr></a:p></c:txPr><c:chart><c:title><c:txPr><a:p><a:pPr><a:defRPr sz="1320"/></a:pPr></a:p></c:txPr></c:title><c:plotArea><c:barChart><c:barDir val="col"/><c:gapWidth val="219"/><c:ser><c:val><c:numRef><c:f>Sheet1!$A$1:$A$6</c:f></c:numRef></c:val></c:ser></c:barChart><c:dateAx><c:numFmt formatCode="yyyy"/><c:majorTickMark val="none"/><c:spPr><a:ln><a:solidFill><a:srgbClr val="D9D9D9"/></a:solidFill></a:ln></c:spPr><c:txPr><a:p><a:pPr><a:defRPr sz="1100"><a:latin typeface="Microsoft JhengHei UI"/></a:defRPr></a:pPr></a:p></c:txPr></c:dateAx><c:valAx><c:numFmt formatCode="General"/><c:majorGridlines/></c:valAx></c:plotArea></c:chart></c:chartSpace>`))
	if err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]*zip.File{}
	for _, f := range zr.File {
		parts[f.Name] = f
	}
	chart := readChart(parts, "xl/charts/chart1.xml")
	if chart == nil || chart.GapWidth == nil || *chart.GapWidth != 219 || chart.FontName != "Microsoft JhengHei UI" || chart.TitleSize != 13.2 || !chart.AutoTitle {
		t.Fatalf("lost chart appearance: %+v", chart)
	}
	axis := chart.CategoryAxis
	if axis == nil || axis.Format != "yyyy" || axis.FontSize != 11 || axis.FontName != chart.FontName || axis.LineColor != "D9D9D9" || axis.MajorTick != "none" {
		t.Fatalf("lost date axis: %+v", axis)
	}
}

func TestColumnGapRoundTrip(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	f.SetCellValue("Sheet1", "A1", 20000)
	gap := uint(219)
	if err := f.AddChart("Sheet1", "D2", &excelize.Chart{Type: excelize.Col, GapWidth: &gap, Series: []excelize.ChartSeries{{Values: "Sheet1!$A$1:$A$1"}}}); err != nil {
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
	chart := wb.Sheets[0].Layout.Objects[0].Chart
	if chart.GapWidth == nil || *chart.GapWidth != 219 {
		t.Fatalf("source gap: %+v", chart)
	}
	layout := model.Layout{Objects: []model.Object{{Chart: chart}}}
	clone := layout.Clone()
	*clone.Objects[0].Chart.GapWidth = 0
	if *chart.GapWidth != 219 {
		t.Fatal("cloned gap aliases original")
	}
	for _, width := range []float64{0, 219} {
		*chart.GapWidth = width
		path := filepath.Join(t.TempDir(), "copy.xlsx")
		saved, err := Save(path, wb, nil, Changes{})
		if err != nil {
			t.Fatal(err)
		}
		saved.Close()
		back, b, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		got := back.Sheets[0].Layout.Objects[0].Chart
		b.Close()
		if got.GapWidth == nil || *got.GapWidth != width {
			t.Fatalf("roundtrip gap %g: %+v", width, got)
		}
	}
}
