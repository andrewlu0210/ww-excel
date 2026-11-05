package service

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"ww-excel/internal/document"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
)

func TestChartLabelEditThroughBridgePreservesOriginal(t *testing.T) {
	d := document.New()
	sh := d.Workbook().Sheets[0]
	sh.Name = "Chart"
	sh.Set(0, 0, model.NumberCell(100))
	sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, W: 400, H: 240, Chart: &model.Chart{Type: "column", Legend: "right", Series: []model.ChartSeries{{Name: `"Sales"`, Values: "Chart!$A$1"}}}}}
	filename := filepath.Join(t.TempDir(), "source.xlsx")
	if err := d.SaveAs(filename); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err := document.Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	svc := NewSheetService(NewSession(d, func(string, any) {}))
	obj := d.Workbook().Sheets[0].Layout.Objects[0]
	dto := objectDTO(obj)
	raw, _ := json.Marshal(dto)
	dto = ObjectDTO{}
	json.Unmarshal(raw, &dto)
	dto.Chart.Series[0].Labels.ShowValue = true
	dto.Chart.Series[0].Labels.FontSize = 16
	text := "Custom"
	dto.Chart.Series[0].Labels.Points = map[int]model.ChartLabelPoint{0: {CustomText: &text, Layout: &model.ChartLabelLayout{X: .45, Y: .25, XMode: "edge", YMode: "edge"}}}
	if err := svc.UpdateObject(bg, 0, dto, "Edit labels"); err != nil {
		t.Fatal(err)
	}
	got := d.Workbook().Sheets[0].Layout.Objects[0].Chart
	if !xlsxfile.CanPreserveChart(got) {
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(got.Source.Baseline)
		t.Fatalf("bridge lost provenance: current=%s baseline=%s", a, b)
	}
	if risks := d.SaveRisks(filepath.Join(t.TempDir(), "copy.xlsx")); len(risks) > 0 {
		t.Fatalf("unexpected risks: %+v", risks)
	}
	svc.Undo(bg)
	if d.Workbook().Sheets[0].Layout.Objects[0].Chart.Series[0].Labels.ShowValue {
		t.Fatal("undo retained edited labels")
	}
	svc.Redo(bg)
	if !d.Workbook().Sheets[0].Layout.Objects[0].Chart.Series[0].Labels.ShowValue {
		t.Fatal("redo lost labels")
	}
}

func TestDraggedChartLabelSaveAndUndo(t *testing.T) {
	for _, typ := range []string{"column", "doughnut"} {
		t.Run(typ, func(t *testing.T) {
			d := document.New()
			sh := d.Workbook().Sheets[0]
			sh.Set(0, 0, model.NumberCell(100))
			sh.Set(1, 0, model.NumberCell(200))
			sh.Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectChart, W: 500, H: 300, Chart: &model.Chart{Type: typ, Legend: "right", Series: []model.ChartSeries{{Values: "A1:A2", Labels: &model.ChartLabels{ShowValue: true, FontSize: 12}}}}}}
			source := filepath.Join(t.TempDir(), "source.xlsx")
			if err := d.SaveAs(source); err != nil {
				t.Fatal(err)
			}
			d.Close()
			d, err := document.Load(source)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			svc := NewSheetService(NewSession(d, func(string, any) {}))
			original := d.Workbook().Sheets[0].Layout.Objects[0]
			if err := svc.MoveChartLabel(bg, 0, original.ID, 0, 1, .43, .27); err != nil {
				t.Fatal(err)
			}
			moved := d.Workbook().Sheets[0].Layout.Objects[0]
			if moved.W != original.W || moved.H != original.H || moved.Anchor != original.Anchor || !xlsxfile.CanPreserveChart(moved.Chart) {
				t.Fatal("changed chart geometry or lost source")
			}
			svc.Undo(bg)
			if d.Workbook().Sheets[0].Layout.Objects[0].Chart.Series[0].Labels.Points[1].Layout != nil {
				t.Fatal("undo retained drag")
			}
			svc.Redo(bg)
			for _, ext := range []string{"xlsx", "wwx"} {
				output := filepath.Join(t.TempDir(), "dragged."+ext)
				if risks := d.SaveRisks(output); len(risks) > 0 {
					t.Fatalf("unexpected warning: %+v", risks)
				}
				if err := d.SaveAs(output); err != nil {
					t.Fatal(err)
				}
				back, err := document.Load(output)
				if err != nil {
					t.Fatal(err)
				}
				p := back.Workbook().Sheets[0].Layout.Objects[0].Chart.Series[0].Labels.Points[1]
				back.Close()
				if p.Layout == nil || p.Layout.X != .43 || p.Layout.Y != .27 || p.Layout.XMode != "edge" {
					t.Fatalf("lost drag in %s: %+v", ext, p)
				}
			}
		})
	}
}
