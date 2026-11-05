package xlsxfile

import (
	"bytes"
	"encoding/xml"
	"github.com/xuri/excelize/v2"
	"path/filepath"
	"reflect"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func TestSheetTabColorsRoundTrip(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	customColor := "80CA6F"
	f.Theme.ThemeElements.ClrScheme.Accent3.SrgbClr.Val = &customColor
	names := []string{"RGB", "Theme", "Indexed", "Plain", "Link", "Visited"}
	f.SetSheetName("Sheet1", names[0])
	for _, name := range names[1:] {
		f.NewSheet(name)
	}
	rgb := "FF123456"
	theme := 6
	tint := -.499984740745262
	indexed := 3
	f.SetSheetProps("RGB", &excelize.SheetPropsOptions{TabColorRGB: &rgb})
	f.SetSheetProps("Theme", &excelize.SheetPropsOptions{TabColorTheme: &theme, TabColorTint: &tint})
	f.SetSheetProps("Indexed", &excelize.SheetPropsOptions{TabColorIndexed: &indexed})
	link, visited := "123ABC", "803399"
	f.Theme.ThemeElements.ClrScheme.Hlink.SrgbClr.Val = &link
	f.Theme.ThemeElements.ClrScheme.FolHlink.SrgbClr.Val = &visited
	linkTheme, visitedTheme := 10, 11
	f.SetSheetProps("Link", &excelize.SheetPropsOptions{TabColorTheme: &linkTheme})
	f.SetSheetProps("Visited", &excelize.SheetPropsOptions{TabColorTheme: &visitedTheme})
	source := filepath.Join(t.TempDir(), "source.xlsx")
	if err := f.SaveAs(source); err != nil {
		t.Fatal(err)
	}
	expected := []string{"123456", normalizeHex(excelize.ThemeColor("80CA6F", tint)), "00FF00", "", link, visited}
	wb, base, err := Read(source)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()
	colors := func(wb *model.Workbook) []string {
		out := []string{}
		for _, sh := range wb.Sheets {
			out = append(out, sh.Layout.TabColor)
		}
		return out
	}
	if got := colors(wb); !reflect.DeepEqual(got, expected) {
		t.Fatalf("source colors: %v, want %v", got, expected)
	}
	old := wb.Sheets[1].Layout.Clone()
	wb.Sheets[1].Layout.Zoom = 125
	edited := filepath.Join(t.TempDir(), "edited.xlsx")
	saved, err := Save(edited, wb, base, Changes{OldLayouts: map[*model.Sheet]model.Layout{wb.Sheets[1]: old}})
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	properties, err := saved.GetSheetProps("Theme")
	if err != nil {
		t.Fatal(err)
	}
	if properties.TabColorTheme == nil || *properties.TabColorTheme != theme || properties.TabColorTint == nil || *properties.TabColorTint != tint {
		t.Fatalf("lost original theme reference: %+v", properties)
	}
	var native bytes.Buffer
	if err := wwx.Write(&native, wb); err != nil {
		t.Fatal(err)
	}
	back, err := wwx.Read(&native)
	if err != nil {
		t.Fatal(err)
	}
	if got := colors(back); !reflect.DeepEqual(got, expected) {
		t.Fatalf("native colors: %v", got)
	}
	output := filepath.Join(t.TempDir(), "native.xlsx")
	nb, err := Save(output, back, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	defer nb.Close()
	for _, filename := range []string{edited, output} {
		got, base, err := Read(filename)
		if err != nil {
			t.Fatal(err)
		}
		base.Close()
		if !reflect.DeepEqual(colors(got), expected) {
			t.Fatalf("%s colors: %v", filename, colors(got))
		}
	}
}
func TestSheetTabInvalidOrAutomaticColors(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	r := reader{f: f}
	for _, raw := range []string{`<tabColor/>`, `<tabColor auto="1"/>`, `<tabColor theme="-1"/>`, `<tabColor theme="99"/>`, `<tabColor indexed="-1"/>`, `<tabColor indexed="64"/>`, `<tabColor rgb="garbage"/>`, `<tabColor rgb="FF123456" tint="NaN"/>`, `<tabColor rgb="FF123456" tint="2"/>`} {
		var node struct {
			Attrs []xml.Attr `xml:",any,attr"`
		}
		if err := xml.Unmarshal([]byte(raw), &node); err != nil {
			t.Fatal(err)
		}
		if got := r.sheetTabColor(xml.StartElement{Attr: node.Attrs}); got != "" {
			t.Fatalf("%s resolved to %s", raw, got)
		}
	}
}

func TestEditAndClearImportedThemeTabColorAcrossRepeatedSaves(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	theme := 6
	tint := .4
	code := "KeepCodeName"
	if err := f.SetSheetProps("Sheet1", &excelize.SheetPropsOptions{TabColorTheme: &theme, TabColorTint: &tint, CodeName: &code}); err != nil {
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
	for _, color := range []string{"FF0000", "", "00AAFF", ""} {
		previous := sh.Layout.Clone()
		sh.Layout.TabColor = color
		output := filepath.Join(t.TempDir(), "changed.xlsx")
		base, err = Save(output, wb, base, Changes{OldLayouts: map[*model.Sheet]model.Layout{sh: previous}})
		if err != nil {
			t.Fatal(err)
		}
		loaded, readBase, err := Read(output)
		if err != nil {
			t.Fatal(err)
		}
		got := loaded.Sheets[0].Layout.TabColor
		readBase.Close()
		if got != color {
			t.Fatalf("color %q became %q", color, got)
		}
		props, err := base.GetSheetProps(sh.Name)
		if err != nil {
			t.Fatal(err)
		}
		if props.TabColorTheme != nil || props.TabColorTint != nil && *props.TabColorTint != 0 {
			t.Fatalf("old theme/tint remains active: %+v", props)
		}
		if props.CodeName == nil || *props.CodeName != code {
			t.Fatal("unrelated sheet properties lost")
		}
	}
}
