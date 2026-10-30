package xlsxfile

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/io/wwx"
)

func TestHiddenSheetsAndDoughnutRoundTrip(t *testing.T) {
	f := excelize.NewFile()
	showGridLines := false
	if err := f.SetSheetView("Sheet1", 0, &excelize.ViewOptions{ShowGridLines: &showGridLines}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Data", "Secret"} {
		if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
	}
	f.SetCellValue("Data", "A1", 25)
	f.SetCellValue("Data", "A2", 75)
	if err := f.SetSheetVisible("Data", false); err != nil {
		t.Fatal(err)
	}
	if err := f.SetSheetVisible("Secret", false, true); err != nil {
		t.Fatal(err)
	}
	if err := f.AddChart("Sheet1", "D2", &excelize.Chart{Type: excelize.Doughnut, HoleSize: 75, Fill: excelize.Fill{Type: "pattern"}, Series: []excelize.ChartSeries{{Values: "Data!$A$1:$A$2"}}}); err != nil {
		t.Fatal(err)
	}
	if err := f.AddChart("Sheet1", "J2", &excelize.Chart{Type: excelize.Col, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D9D9D0"}}, Series: []excelize.ChartSeries{{Values: "Data!$A$1:$A$2"}}}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "source.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	f.Close()
	wb, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	check := func(path string) {
		t.Helper()
		got, b, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		defer b.Close()
		if !got.Sheets[0].Visible() || got.Sheets[1].Visibility != "hidden" || got.Sheets[2].Visibility != "veryHidden" {
			t.Fatalf("lost visibility: %+v", got.Sheets)
		}
		if !got.Sheets[0].Layout.HideGridLines {
			t.Fatal("lost hidden gridlines setting")
		}
		chart := got.Sheets[0].Layout.Objects[0].Chart
		if chart == nil || chart.Type != "doughnut" || chart.HoleSize != 75 || chart.Background != "transparent" {
			t.Fatalf("lost doughnut: %+v", chart)
		}
		if got.Sheets[0].Layout.Objects[1].Chart.Background != "D9D9D0" {
			t.Fatal("lost solid chart background")
		}

	}
	check(path)
	for _, source := range []bool{true, false} {
		out := filepath.Join(t.TempDir(), "copy.xlsx")
		var src *excelize.File
		if source {
			src = base
		}
		saved, err := Save(out, wb, src, Changes{})
		if err != nil {
			t.Fatal(err)
		}
		if !source {
			saved.Close()
		}
		check(out)
	}
	var buf bytes.Buffer
	if err := wwx.Write(&buf, wb); err != nil {
		t.Fatal(err)
	}
	restored, err := wwx.Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Sheets[0].Layout.HideGridLines {
		t.Fatal("native format lost hidden gridlines setting")
	}
	if restored.Sheets[1].Visibility != "hidden" || restored.Sheets[2].Visibility != "veryHidden" || restored.Sheets[0].Layout.Objects[0].Chart.HoleSize != 75 || restored.Sheets[0].Layout.Objects[0].Chart.Background != "transparent" {
		t.Fatal("native format lost visibility/chart settings")
	}
}
