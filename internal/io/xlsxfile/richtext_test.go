package xlsxfile

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/model"
)

func TestReadRichTextElement(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	for _, tag := range []string{"si", "is"} {
		x := `<` + tag + `><r><t xml:space="preserve">標題&#10;&#10;</t></r><r><rPr><sz val="12"/><color rgb="FF00B0F0"/><rFont val="微软雅黑"/></rPr><t>指標庫：系統保障</t></r><rPh><t>not part of the value</t></rPh></` + tag + `>`
		dec := xml.NewDecoder(strings.NewReader(x))
		dec.RawToken()
		text, rich, err := readTextElement(dec, tag, f)
		if err != nil {
			t.Fatal(err)
		}
		if text != "標題\n\n指標庫：系統保障" || rich == nil || len(rich.Runs) != 2 {
			t.Fatalf("text/runs: %q %+v", text, rich)
		}
		if rich.Runs[1].Font.Resolve(model.DefaultFont).Color != "00B0F0" {
			t.Fatal("lost rich font color")
		}
	}
}

func TestRichTextXLSXRoundTrip(t *testing.T) {
	f := excelize.NewFile()
	if err := f.SetCellRichText("Sheet1", "C4", []excelize.RichTextRun{{Text: "標題\n\n"}, {Text: "藍字", Font: &excelize.Font{Color: "00B0F0", Size: 12, Family: "微软雅黑"}}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "rich.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	f.Close()
	wb, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	cell := wb.Sheets[0].Get(3, 2)
	if cell.Rich == nil || cell.Str != "標題\n\n藍字" {
		t.Fatalf("rich text missing: %+v", cell)
	}
	copy := filepath.Join(t.TempDir(), "copy.xlsx")
	if _, err := Save(copy, wb, nil, Changes{}); err != nil {
		t.Fatal(err)
	}
	back, backBase, err := Read(copy)
	if err != nil {
		t.Fatal(err)
	}
	defer backBase.Close()
	if got := back.Sheets[0].Get(3, 2); got.Rich == nil || got.Rich.Runs[1].Font.Resolve(model.DefaultFont).Color != "00B0F0" {
		t.Fatalf("round trip lost rich formatting: %+v", got)
	}
}

// The user-provided workbook is optional locally; synthetic tests remain portable.
func TestUserWorkbookDisplayCompatibility(t *testing.T) {
	path := filepath.Join("..", "..", "..", "test.xlsx")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("local user workbook not present")
	}
	wb, base, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	if len(wb.Sheets) != 8 {
		t.Skip("local fixture is not the original eight-sheet compatibility workbook")
	}
	for _, sh := range wb.Sheets {
		found := false
		for _, o := range sh.Layout.Objects {
			if o.Shape == nil {
				continue
			}
			var title strings.Builder
			for _, r := range o.Shape.Text.Runs {
				title.WriteString(r.Text)
			}
			if strings.Contains(title.String(), "组织绩效设定与评估") {
				found = true
				if o.Shape.Geometry != "roundRect" || o.Shape.Fill == "" || o.Shape.Radius <= 0 {
					t.Fatalf("shape styling missing: %+v", o.Shape)
				}
				t.Logf("%s: title=%q fill=%s font=%+v size=%gx%g", sh.Name, title.String(), o.Shape.Fill, o.Shape.Text.Runs[len(o.Shape.Text.Runs)-1].Font.Resolve(o.Shape.Font), o.W, o.H)
			}
		}
		if !found {
			t.Fatalf("missing heading in %s", sh.Name)
		}
	}
	for row := 3; row <= 5; row++ {
		cell := wb.Sheets[3].Get(row, 2)
		if cell.Rich == nil || len(cell.Rich.Runs) != 2 {
			t.Fatalf("C%d rich runs missing", row+1)
		}
		font := cell.Rich.Runs[1].Font.Resolve(wb.StyleTable().Get(wb.Sheets[3].EffectiveStyle(row, 2)).Font)
		if font.Color != "00B0F0" {
			t.Fatalf("C%d color=%s", row+1, font.Color)
		}
		t.Logf("C%d: %q color=%s", row+1, cell.Str, font.Color)
	}
	// Save a copy with a changed rich-text cell and preserve the original drawings.
	sh := wb.Sheets[3]
	copyPath := filepath.Join(t.TempDir(), "copy.xlsx")
	if _, err := Save(copyPath, wb, base, Changes{Cells: map[*model.Sheet]map[[2]int]bool{sh: {{3, 2}: true}}}); err != nil {
		t.Fatal(err)
	}
	back, backBase, err := Read(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer backBase.Close()
	cell := back.Sheets[3].Get(3, 2)
	if cell.Rich == nil || len(cell.Rich.Runs) != 2 || cell.Rich.Runs[1].Font.Resolve(model.DefaultFont).Color != "00B0F0" {
		t.Fatal("saved copy lost rich text")
	}
	foundShape := false
	for _, o := range back.Sheets[3].Layout.Objects {
		foundShape = foundShape || o.Shape != nil
	}
	if !foundShape {
		t.Fatal("saved copy lost title shape")
	}
}
