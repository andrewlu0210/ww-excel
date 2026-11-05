package xlsxfile

import (
	"path/filepath"
	"strings"
	"testing"
	"ww-excel/internal/model"
)

// CT_ValAx excludes the category-axis suffix (ISO/IEC 29500, A.5.1).
func assertValueAxisStructure(t *testing.T, data []byte) {
	t.Helper()
	root, err := drawingNodes(data)
	if err != nil {
		t.Fatal(err)
	}
	order := strings.Fields("axId scaling delete axPos majorGridlines minorGridlines title numFmt majorTickMark minorTickMark tickLblPos spPr txPr crossAx crosses crossesAt crossBetween majorUnit minorUnit dispUnits extLst")
	ranks := map[string]int{}
	for i, name := range order {
		ranks[name] = i
	}
	for _, axis := range root.all("valAx") {
		previous := -1
		for _, child := range axis.children {
			rank, ok := ranks[child.name]
			if !ok || rank < previous {
				t.Fatalf("invalid value-axis child %q: %s", child.name, axis.raw(data))
			}
			previous = rank
		}
		for _, name := range []string{"axId", "scaling", "axPos", "crossAx"} {
			if axis.child(name) == nil {
				t.Fatalf("missing %s", name)
			}
		}
	}
}

func TestScatterExportValueAxisStructure(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	for r := 0; r < 3; r++ {
		sh.Set(r, 0, model.NumberCell(float64(r)))
		sh.Set(r, 1, model.NumberCell(float64(r*2)))
	}
	sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, W: 500, H: 300, Chart: &model.Chart{Type: "scatter", Series: []model.ChartSeries{{Categories: "A1:A3", Values: "B1:B3"}}}}}
	filename := filepath.Join(t.TempDir(), "scatter.xlsx")
	f, err := Save(filename, wb, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	loaded, source, err := Read(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	assertValueAxisStructure(t, []byte(loaded.Sheets[0].Layout.Objects[0].Chart.Source.XML))
	// Simulate an untouched chart part from a previous release. Saving a cell
	// edit or an unchanged workbook must repair it without rebuilding charts.
	raw, ok := source.Pkg.Load("xl/charts/chart1.xml")
	if !ok {
		t.Fatal("missing chart part")
	}
	broken := strings.ReplaceAll(string(raw.([]byte)), "</valAx>", `<auto val="1"/><lblAlgn val="ctr"/><lblOffset val="100"/><noMultiLvlLbl val="0"/></valAx>`)
	source.Pkg.Store("xl/charts/chart1.xml", []byte(broken))
	repaired := filepath.Join(t.TempDir(), "repaired.xlsx")
	saved, err := Save(repaired, loaded, source, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	saved.Close()
	back, file, err := Read(repaired)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	assertValueAxisStructure(t, []byte(back.Sheets[0].Layout.Objects[0].Chart.Source.XML))
}

func TestRepairPreviouslyExportedScatterAxisPreservesNumericSettings(t *testing.T) {
	raw := []byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea><c:valAx><c:axId val="1"/><c:scaling><c:min val="-1"/><c:max val="10"/></c:scaling><c:axPos val="b"/><c:crossAx val="2"/><c:majorUnit val="2"/><c:auto val="1"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/><c:tickLblSkip val="1"/><c:tickMarkSkip val="1"/><c:noMultiLvlLbl val="0"/><c:extLst><c:ext uri="keep"/></c:extLst></c:valAx></c:plotArea></c:chart></c:chartSpace>`)
	fixed := repairChartValueAxes(raw)
	assertValueAxisStructure(t, fixed)
	for _, token := range []string{`val="-1"`, `val="10"`, `val="2"`, `uri="keep"`} {
		if !strings.Contains(string(fixed), token) {
			t.Fatalf("lost %s", token)
		}
	}
	if string(repairChartValueAxes(fixed)) != string(fixed) {
		t.Fatal("repair must be idempotent")
	}
}
