package document

import (
	"path/filepath"
	"testing"

	"ww-excel/internal/model"
)

func TestCSVSaveRisks(t *testing.T) {
	d := New()
	enter(t, d, 0, 0, 0, "plain text")
	if risks := d.SaveRisks("plain.csv"); len(risks) != 0 {
		t.Fatalf("plain CSV: %+v", risks)
	}
	enter(t, d, 0, 1, 0, "=1+2")
	enter(t, d, 0, 2, 0, "12.5%")
	sh := d.wb.Sheets[0]
	sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, Chart: &model.Chart{Type: "column"}}}
	sh.Layout.Notes = map[[2]int]model.Note{{0, 0}: {Text: "note"}}
	d.wb.Sheets = append(d.wb.Sheets, model.NewSheet("隱藏資料"))
	d.wb.Sheets[1].Visibility = "hidden"
	want := map[string]bool{"csv-sheets": true, "csv-formulas": true, "csv-formatting": true, "csv-objects": true, "csv-settings": true}
	for _, risk := range d.SaveRisks("output.CSV") {
		if !want[risk.Code] || risk.Sheet != sh.Name || risk.Message == "" {
			t.Fatalf("unexpected risk: %+v", risk)
		}
		delete(want, risk.Code)
	}
	if len(want) != 0 {
		t.Fatalf("missing risks: %v", want)
	}
	if risks := d.SaveRisks("output.xlsx"); len(risks) != 0 {
		t.Fatalf("new XLSX: %+v", risks)
	}
	if risks := d.SaveRisks("output.wwx"); len(risks) != 0 {
		t.Fatalf("native: %+v", risks)
	}
}

func TestImportedChartSaveRisks(t *testing.T) {
	d := New()
	enter(t, d, 0, 0, 0, "10")
	chart := &model.Chart{Type: "column", Title: "Sales", Series: []model.ChartSeries{{Values: "A1"}}}
	cmd, id, err := d.AddObjectCommand(0, model.Object{Kind: model.ObjectChart, W: 300, H: 200, Chart: chart})
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	path := filepath.Join(t.TempDir(), "source.xlsx")
	if err := d.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	obj := d.wb.Sheets[0].Layout.Objects[0]
	obj.W += 20
	cmd, err = d.UpdateObjectCommand(0, obj, "resize")
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	if got := d.SaveRisks(path); len(got) != 0 {
		t.Fatalf("app-created chart must not warn: %+v", got)
	}
	d.Close()
	d, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	enter(t, d, 0, 1, 0, "20")
	if got := d.SaveRisks(path); len(got) != 0 {
		t.Fatalf("cell edit must preserve chart: %+v", got)
	}
	obj = d.wb.Sheets[0].Layout.Objects[0]
	obj.W += 50
	cmd, err = d.UpdateObjectCommand(0, obj, "resize")
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	if got := d.SaveRisks(path); len(got) != 0 {
		t.Fatalf("resizing must preserve original chart XML: %+v", got)
	}
	obj = d.wb.Sheets[0].Layout.Clone().Objects[0]
	obj.Chart.Type = "line"
	cmd, err = d.UpdateObjectCommand(0, obj, "change chart type")
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	for _, output := range []string{path, filepath.Join(t.TempDir(), "copy.xlsx")} {
		risks := d.SaveRisks(output)
		if len(risks) != 1 || risks[0].Code != "xlsx-chart" || risks[0].ObjectID != id {
			t.Fatalf("chart edit: %+v", risks)
		}
	}
	d.Undo()
	if got := d.SaveRisks(path); len(got) != 0 {
		t.Fatalf("undo must remove risk: %+v", got)
	}
}
