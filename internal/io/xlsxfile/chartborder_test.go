package xlsxfile

import (
	"github.com/xuri/excelize/v2"
	"path/filepath"
	"testing"
	"ww-excel/internal/model"
)

func TestReadChartDefaultAndExplicitBorders(t *testing.T) {
	for _, tc := range []struct{ name, fill, want string }{{"default", "", "D9D9D9"}, {"none", `<a:noFill/>`, ""}, {"color", `<a:solidFill><a:srgbClr val="AA0000"/></a:solidFill>`, "AA0000"}} {
		t.Run(tc.name, func(t *testing.T) {
			f := excelize.NewFile()
			defer f.Close()
			f.SetCellValue("Sheet1", "A1", 10)
			if err := f.AddChart("Sheet1", "D2", &excelize.Chart{Type: excelize.Col, Series: []excelize.ChartSeries{{Values: "Sheet1!$A$1"}}}); err != nil {
				t.Fatal(err)
			}
			if tc.fill != "" {
				raw, _ := f.Pkg.Load("xl/charts/chart1.xml")
				data := raw.([]byte)
				root, _ := drawingNodes(data)
				props := chartElement("spPr", `<a:ln>`+tc.fill+`</a:ln>`)
				f.Pkg.Store("xl/charts/chart1.xml", []byte(replaceChartChildren(data, root.children[0], []chartReplacement{{"spPr", props}}, nil)))
			}
			path := filepath.Join(t.TempDir(), "chart.xlsx")
			if err := f.SaveAs(path); err != nil {
				t.Fatal(err)
			}
			wb, source, err := Read(path)
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			if got := wb.Sheets[0].Layout.Objects[0].Chart.Border; got != tc.want {
				t.Fatalf("border %q want %q", got, tc.want)
			}
		})
	}
}
func TestGeneratedChartBorderRoundTrip(t *testing.T) {
	for _, color := range []string{"", "D9D9D9", "AA0000"} {
		wb := model.NewWorkbook()
		sh := wb.Sheets[0]
		sh.Set(0, 0, model.NumberCell(10))
		sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, W: 400, H: 250, Chart: &model.Chart{Type: "column", Border: color, Series: []model.ChartSeries{{Values: "A1"}}}}}
		path := filepath.Join(t.TempDir(), "chart.xlsx")
		f, err := Save(path, wb, nil, Changes{})
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		back, source, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		source.Close()
		if got := back.Sheets[0].Layout.Objects[0].Chart.Border; got != color {
			t.Fatalf("border %q want %q", got, color)
		}
	}
}
