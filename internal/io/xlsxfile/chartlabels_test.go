package xlsxfile

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/model"
)

func TestChartLabelAndColorRoundTrip(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	f.SetCellValue("Sheet1", "A1", .38)
	f.SetCellValue("Sheet1", "A2", .62)
	if err := f.SetDefinedName(&excelize.DefinedName{Name: "Ratio", RefersTo: "'Sheet1'!$A$2"}); err != nil {
		t.Fatal(err)
	}
	if err := f.AddChart("Sheet1", "C1", &excelize.Chart{Type: excelize.Doughnut, Series: []excelize.ChartSeries{{Values: "Sheet1!$A$1:$A$2"}}}); err != nil {
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
	c.Series[0].CategoryLabels = []string{"", "A&B"}
	c.Series[0].Color = "0592FE"
	c.Series[0].PointColors = map[int]string{0: "69BBFE", 1: "0592FE"}
	c.Series[0].Labels = &model.ChartLabels{RawXML: `<c:dLbl><c:idx val="0"/><c:delete val="1"/></c:dLbl><c:dLbl><c:idx val="1"/><c:extLst><c:ext uri="test"><c15:layout><c:manualLayout><c:w val="0.99"/><c:h val="1"/></c:manualLayout></c15:layout></c:ext></c:extLst></c:dLbl><c:numFmt formatCode="0%"/><c:showVal val="1"/><c:txPr><a:p><a:pPr><a:defRPr sz="5300"><a:solidFill><a:srgbClr val="666666"/></a:solidFill></a:defRPr></a:pPr></a:p></c:txPr>`}
	out := filepath.Join(t.TempDir(), "copy.xlsx")
	saved, err := Save(out, wb, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	saved.Close()
	back, b, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	s := back.Sheets[0].Layout.Objects[0].Chart.Series[0]
	if s.Color != "0592FE" || s.PointColors[0] != "69BBFE" || s.PointColors[1] != "0592FE" {
		t.Fatalf("lost colors: %+v", s)
	}
	if len(s.CategoryLabels) != 2 || s.CategoryLabels[0] != "" || s.CategoryLabels[1] != "A&B" {
		t.Fatalf("lost literal category labels: %#v", s.CategoryLabels)
	}
	l := s.Labels
	if l == nil || l.Format != "0%" || !l.ShowValue || l.FontSize != 53 || l.Color != "666666" || !l.Points[0].Hidden || !l.Points[1].Center {
		t.Fatalf("lost labels: %+v", l)
	}
	preserved, err := Save(filepath.Join(t.TempDir(), "with-source.xlsx"), wb, base, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	for _, dn := range preserved.GetDefinedName() {
		if dn.Name == "Ratio" && (strings.Contains(dn.RefersTo, "__ww_") || !strings.Contains(dn.RefersTo, "Sheet1")) {
			t.Fatalf("temporary sheet name escaped into saved formula: %+v", dn)
		}
	}

}
