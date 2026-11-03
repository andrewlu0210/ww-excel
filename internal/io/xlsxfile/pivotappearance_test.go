package xlsxfile

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func TestCachedPivotAppearanceAndCustomTableFills(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range map[string]string{
		"xl/styles.xml":                       `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dxfs count="4"><dxf><font><color theme="0"/></font><fill><patternFill><bgColor rgb="FF339966"/></patternFill></fill></dxf><dxf><fill><patternFill><bgColor theme="0" tint="-0.05"/></patternFill></fill></dxf><dxf><fill><patternFill patternType="solid"><fgColor rgb="FF359966"/></patternFill></fill><font><color theme="0"/></font></dxf><dxf><fill><patternFill patternType="none"/></fill></dxf></dxfs><tableStyles><tableStyle name="Custom"><tableStyleElement type="headerRow" dxfId="0"/><tableStyleElement type="firstRowStripe" size="2" dxfId="1"/><tableStyleElement type="secondRowStripe" dxfId="3"/></tableStyle><tableStyle name="Pivot"><tableStyleElement type="headerRow" dxfId="2"/><tableStyleElement type="totalRow" dxfId="2"/></tableStyle></tableStyles></styleSheet>`,
		"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotTable" Target="../pivotTables/pivotTable1.xml"/></Relationships>`,
		"xl/pivotTables/pivotTable1.xml":      `<pivotTableDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><location ref="E11:F15" firstDataRow="1" firstDataCol="1"/><rowItems><i/><i/><i/><i t="grand"/></rowItems><pivotTableStyleInfo name="Pivot"/></pivotTableDefinition>`,
	} {
		out, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = out.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]*zip.File{}
	for _, f := range zr.File {
		parts[f.Name] = f
	}
	f := excelize.NewFile()
	defer f.Close()
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	inherited := wb.StyleTable().Default()
	inherited.NumFmt = `"NT$"#,##0`
	inherited.Font.Name = "Arial"
	sh.Layout.ColStyles[5] = wb.StyleTable().Intern(inherited)
	sh.Set(10, 4, model.StringCell("列標籤"))
	sh.Set(10, 5, model.StringCell("加總 - 金額"))
	sh.Set(11, 4, model.StringCell("啤酒"))
	sh.Set(11, 5, model.NumberCell(2150))
	sh.Set(14, 4, model.StringCell("總計"))
	sh.Set(14, 5, model.NumberCell(5950))
	r := reader{f: f, parts: parts, wb: wb}
	a := r.tableAppearance("Custom")
	if a == nil || a.HeaderFill == nil || *a.HeaderFill != "339966" || a.FirstStripeFill == nil || *a.FirstStripeFill != "F2F2F2" || a.FirstStripeSize != 2 || a.SecondStripeFill == nil || *a.SecondStripeFill != "" {
		t.Fatalf("table fills: %+v", a)
	}
	r.readPivotAppearance(sh, "xl/worksheets/sheet1.xml")
	if len(sh.Layout.Tables) != 0 || len(sh.Layout.PivotHeaders) != 1 || sh.Layout.PivotHeaders[0] != (model.Range{R0: 10, C0: 4, R1: 10, C1: 4}) {
		t.Fatalf("pivot header metadata: %+v", sh.Layout)
	}
	for _, pos := range [][2]int{{10, 4}, {10, 5}, {14, 4}, {14, 5}} {
		style := wb.StyleTable().Get(sh.Get(pos[0], pos[1]).S)
		if style.Fill != "359966" || style.Font.Color != "FFFFFF" {
			t.Fatalf("pivot style at %v: %+v", pos, style)
		}
		if pos[1] == 5 && (style.NumFmt != inherited.NumFmt || style.Font.Name != "Arial") {
			t.Fatalf("lost inherited number/font format: %+v", style)
		}
	}
	if sh.Get(11, 5).Num != 2150 || sh.Get(14, 5).Num != 5950 || sh.Get(11, 5).S != 0 {
		t.Fatal("changed cached values or body style")
	}
	var native bytes.Buffer
	if err := wwx.Write(&native, wb); err != nil {
		t.Fatal(err)
	}
	back, err := wwx.Read(&native)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Sheets[0].Layout.PivotHeaders) != 1 || back.StyleTable().Get(back.Sheets[0].Get(14, 5).S).Fill != "359966" {
		t.Fatal("native format lost cached pivot appearance")
	}
	clone := sh.Layout.Clone()
	clone.PivotHeaders[0].C0 = 0
	if sh.Layout.PivotHeaders[0].C0 != 4 {
		t.Fatal("cloned pivot headers alias original")
	}
}
