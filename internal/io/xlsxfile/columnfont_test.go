package xlsxfile

import (
	"bytes"
	"github.com/xuri/excelize/v2"
	"math"
	"path/filepath"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func TestNormalFontColumnWidthAndZoom(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 16, Bold: true}})
	if err != nil {
		t.Fatal(err)
	}
	// The Normal style can differ from the default cell XF.
	f.Styles.CellStyleXfs.Xf[0] = f.Styles.CellXfs.Xf[style]
	f.SetColWidth("Sheet1", "B", "B", 26.42578125)
	zoom := 125.0
	f.SetSheetView("Sheet1", 0, &excelize.ViewOptions{ZoomScale: &zoom})
	path := filepath.Join(t.TempDir(), "font.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	wb, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	layout := &wb.Sheets[0].Layout
	mdw := fontDigitWidth(model.Font{Name: "Arial", Size: 16, Bold: true})
	if math.Abs(layout.ColumnDigitWidth-mdw) > 1e-9 || layout.ColWidth(1) != math.Round(26.42578125*mdw) {
		t.Fatalf("wrong font metrics: %+v", layout)
	}
	if layout.Zoom != 125 {
		t.Fatalf("zoom %d", layout.Zoom)
	}
	old := layout.Clone()
	layout.Cols[1] = model.Size{Px: 300}
	layout.DefaultCol = 101
	out := filepath.Join(t.TempDir(), "resized.xlsx")
	saved, err := Save(out, wb, base, Changes{OldLayouts: map[*model.Sheet]model.Layout{wb.Sheets[0]: old}})
	if err != nil {
		t.Fatal(err)
	}
	saved.Close()
	back, b, err := Read(out)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if back.Sheets[0].Layout.DefaultCol != 101 {
		t.Fatalf("default width=%v", back.Sheets[0].Layout.DefaultCol)
	}
	if back.Sheets[0].Layout.ColWidth(1) != 300 {
		t.Fatalf("resized width=%v", back.Sheets[0].Layout.ColWidth(1))
	}
}

func TestColumnWidthsAcrossNativeAndExcelCopies(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	normal, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 16, Bold: true}})
	if err != nil {
		t.Fatal(err)
	}
	f.Styles.CellStyleXfs.Xf[0] = f.Styles.CellXfs.Xf[normal]
	defaultWidth := 9.0
	if err := f.SetSheetProps("Sheet1", &excelize.SheetPropsOptions{DefaultColWidth: &defaultWidth}); err != nil {
		t.Fatal(err)
	}
	f.SetColWidth("Sheet1", "B", "B", 308/fontDigitWidth(model.Font{Name: "Arial", Size: 16, Bold: true}))
	f.SetColVisible("Sheet1", "B", false)
	source := filepath.Join(t.TempDir(), "source.xlsx")
	if err := f.SaveAs(source); err != nil {
		t.Fatal(err)
	}
	wb, base, err := Read(source)
	if err != nil {
		t.Fatal(err)
	}
	base.Close()
	if wb.StyleTable().Default().Font.Name != "Calibri" {
		t.Fatal("fixture default cell font should remain Calibri")
	}
	wantDefault, wantB := wb.Sheets[0].Layout.DefaultCol, wb.Sheets[0].Layout.Cols[1].Px
	if wantB != 308 {
		t.Fatalf("fixture width=%v want 308", wantB)
	}
	for copyIndex := 0; copyIndex < 3; copyIndex++ {
		var native bytes.Buffer
		if err := wwx.Write(&native, wb); err != nil {
			t.Fatal(err)
		}
		wb, err = wwx.Read(&native)
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(t.TempDir(), "copy.xlsx")
		saved, err := Save(out, wb, nil, Changes{})
		if err != nil {
			t.Fatal(err)
		}
		saved.Close()
		wb, base, err = Read(out)
		if err != nil {
			t.Fatal(err)
		}
		base.Close()
		l := wb.Sheets[0].Layout
		if l.Cols[1].Px != wantB || l.DefaultCol != wantDefault || !l.Cols[1].Hidden {
			t.Fatalf("copy %d: B width=%v want %v; default=%v want %v; hidden=%v", copyIndex, l.Cols[1].Px, wantB, l.DefaultCol, wantDefault, l.Cols[1].Hidden)
		}
		if wb.StyleTable().Default().Font.Name != "Calibri" {
			t.Fatal("export changed default cell font")
		}
	}
}

func TestNativeColumnWidthsWithoutOriginalFontMetrics(t *testing.T) {
	wb := model.NewWorkbook()
	style := wb.StyleTable().Default()
	style.Font = model.Font{Name: "Arial", Size: 16, Bold: true}
	wb.Styles = model.NewStyleTable(style)
	wb.Sheets[0].Layout.DefaultCol = 91
	wb.Sheets[0].Layout.Cols[1] = model.Size{Px: 308}
	var native bytes.Buffer
	if err := wwx.Write(&native, wb); err != nil {
		t.Fatal(err)
	}
	back, err := wwx.Read(&native)
	if err != nil {
		t.Fatal(err)
	}
	if back.Sheets[0].Layout.ColumnDigitWidth != 0 {
		t.Fatal("fixture should not contain original column font metrics")
	}
	path := filepath.Join(t.TempDir(), "native.xlsx")
	saved, err := Save(path, back, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	saved.Close()
	got, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	if l := got.Sheets[0].Layout; l.ColWidth(1) != 308 || l.DefaultCol != 91 {
		t.Fatalf("wrong output widths: default=%v B=%v", l.DefaultCol, l.ColWidth(1))
	}
}
