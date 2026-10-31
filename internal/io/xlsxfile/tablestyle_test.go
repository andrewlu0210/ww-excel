package xlsxfile

import (
	"archive/zip"
	"bytes"
	"github.com/xuri/excelize/v2"
	"testing"
)

func TestCustomTableFontOverridesArePartial(t *testing.T) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	part, err := w.Create("xl/styles.xml")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte(`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dxfs count="3"><dxf><font><b val="0"/><color rgb="FF1C639E"/></font><border><horizontal style="medium"><color rgb="FFD9D9D9"/></horizontal></border></dxf><dxf><font><b/><color rgb="FF262626"/></font><border><bottom style="medium"><color auto="1"/></bottom></border></dxf><dxf><font><color rgb="FF7F7F7F"/></font></dxf></dxfs><tableStyles><tableStyle name="Custom"><tableStyleElement type="wholeTable" dxfId="0"/><tableStyleElement type="headerRow" dxfId="1"/><tableStyleElement type="firstColumn" dxfId="2"/></tableStyle></tableStyles></styleSheet>`))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(b.Bytes()), int64(b.Len()))
	if err != nil {
		t.Fatal(err)
	}
	f := excelize.NewFile()
	defer f.Close()
	r := reader{f: f, parts: map[string]*zip.File{"xl/styles.xml": z.File[0]}}
	a := r.tableAppearance("Custom")
	if a == nil || a.Whole == nil || a.Whole.Color == nil || *a.Whole.Color != "1C639E" || a.Whole.Bold == nil || *a.Whole.Bold || a.Whole.Name != nil {
		t.Fatalf("whole font: %+v", a)
	}
	if a.Header == nil || a.Header.Bold == nil || !*a.Header.Bold || a.HeaderBorder.Style != "medium" {
		t.Fatalf("header: %+v", a)
	}
	if a.First == nil || a.First.Color == nil || *a.First.Color != "7F7F7F" || a.First.Bold != nil {
		t.Fatalf("first column: %+v", a.First)
	}
	if got := r.tableRowBorder("Custom"); got.Style != "medium" || got.Color != "D9D9D9" {
		t.Fatalf("row border: %+v", got)
	}
}
