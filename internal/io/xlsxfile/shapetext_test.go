package xlsxfile

import (
	"archive/zip"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/model"
)

func TestShapeTextSurgicalPatchAndNativeRoundTrip(t *testing.T) {
	file := filepath.Join(t.TempDir(), "shape.xlsx")
	f := excelize.NewFile()
	if err := f.AddShape("Sheet1", &excelize.Shape{Cell: "C4", Type: "roundRect", Width: 260, Height: 120, Fill: excelize.Fill{Color: []string{"39734D"}}, Paragraph: []excelize.RichTextRun{{Text: "標題\n內文", Font: &excelize.Font{Size: 16, Color: "FFFFFF", Bold: true}}}}); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(file); err != nil {
		t.Fatal(err)
	}
	f.Close()
	wb, src, err := Read(file)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	sh := wb.Sheets[0]
	if len(sh.Layout.Objects) != 1 || sh.Layout.Objects[0].Shape.RawXML == "" || sh.Layout.Objects[0].Shape.Fill != "39734D" {
		t.Fatal("missing shape provenance")
	}
	old := sh.Layout.Clone()
	obj := sh.Layout.Objects[0]
	shape := *obj.Shape
	red, bold := "FF0000", true
	shape.Text = &model.RichText{Runs: []model.TextRun{{Text: "修改🙂", Font: &model.RichFont{Color: &red, Bold: &bold}}, {Text: "\n\n123 = 文字"}}}
	obj.Shape = &shape
	sh.Layout.Objects[0] = obj
	output := filepath.Join(t.TempDir(), "edited.xlsx")
	if _, err = Save(output, wb, src, Changes{OldLayouts: map[*model.Sheet]model.Layout{sh: old}}); err != nil {
		t.Fatal(err)
	}
	original := drawingPartForTest(t, file)
	updated := drawingPartForTest(t, output)
	a, _ := drawingNodes(original)
	b, _ := drawingNodes(updated)
	left, right := a.all("sp")[0], b.all("sp")[0]
	for _, name := range []string{"nvSpPr", "spPr", "style"} {
		if left.child(name).raw(original) != right.child(name).raw(updated) {
			t.Fatalf("changed %s", name)
		}
	}
	back, other, err := Read(output)
	if err != nil {
		t.Fatal(err)
	}
	other.Close()
	got := back.Sheets[0].Layout.Objects[0]
	if textOfShape(got) != "修改🙂\n\n123 = 文字" || got.Shape.Text.Runs[0].Font.Resolve(got.Shape.Font).Color != "FF0000" || got.Anchor != obj.Anchor || got.W != obj.W || got.H != obj.H {
		t.Fatalf("roundtrip lost shape: %+v text %q", got, textOfShape(got))
	}
	// A new workbook (the same path used by WWX -> XLSX) retains raw appearance.
	out2 := filepath.Join(t.TempDir(), "new-source.xlsx")
	fresh, err := Save(out2, wb, nil, Changes{})
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	reloaded, tmp, err := Read(out2)
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	g := reloaded.Sheets[0].Layout.Objects[0]
	if textOfShape(g) != textOfShape(got) || g.Shape.Fill != got.Shape.Fill || g.Shape.Geometry != got.Shape.Geometry || g.Anchor != got.Anchor {
		t.Fatalf("new source lost shape %q %+v", textOfShape(g), g)
	}
	nextOld := sh.Layout.Clone()
	next := *sh.Layout.Objects[0].Shape
	next.Text = &model.RichText{Runs: []model.TextRun{{Text: "第二次"}}}
	sh.Layout.Objects[0].Shape = &next
	if _, err = Save(out2, wb, fresh, Changes{OldLayouts: map[*model.Sheet]model.Layout{sh: nextOld}}); err != nil {
		t.Fatal(err)
	}
	final, last, err := Read(out2)
	if err != nil {
		t.Fatal(err)
	}
	last.Close()
	if len(final.Sheets[0].Layout.Objects) != 1 || textOfShape(final.Sheets[0].Layout.Objects[0]) != "第二次" {
		t.Fatal("repeated save duplicated/lost shape")
	}
	if reflect.DeepEqual(old.Objects, sh.Layout.Objects) {
		t.Fatal("fixture did not edit shape")
	}
}
func textOfShape(obj model.Object) string {
	var b strings.Builder
	if obj.Shape.Text != nil {
		for _, r := range obj.Shape.Text.Runs {
			b.WriteString(r.Text)
		}
	}
	return b.String()
}
func drawingPartForTest(t *testing.T, file string) []byte {
	t.Helper()
	z, err := zip.OpenReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	parts := map[string]*zip.File{}
	for _, p := range z.File {
		parts[p.Name] = p
	}
	return readPart(parts, "xl/drawings/drawing1.xml")
}

func TestGroupShapeEditLeavesSiblingAndPropertiesUntouched(t *testing.T) {
	source := []byte(`<xdr:wsDr xmlns:xdr="` + spreadsheetDrawingNS + `" xmlns:a="` + drawingNS + `"><xdr:twoCellAnchor><xdr:grpSp><xdr:nvGrpSpPr/><xdr:grpSpPr><a:xfrm rot="120000"><a:off x="10" y="20"/><a:ext cx="300" cy="400"/><a:chOff x="0" y="0"/><a:chExt cx="600" cy="800"/></a:xfrm></xdr:grpSpPr><xdr:sp><xdr:nvSpPr><xdr:cNvPr id="7" name="Text"/></xdr:nvSpPr><xdr:spPr><a:solidFill><a:srgbClr val="39734D"/></a:solidFill></xdr:spPr><xdr:txBody><a:bodyPr lIns="12000" wrap="square"/><a:lstStyle/><a:p><a:pPr algn="ctr"><a:spcAft><a:spcPts val="800"/></a:spcAft></a:pPr><a:r><a:t>原始文字</a:t></a:r></a:p></xdr:txBody></xdr:sp><xdr:sp><xdr:nvSpPr><xdr:cNvPr id="8" name="Sibling"/></xdr:nvSpPr><xdr:spPr/><xdr:txBody><a:bodyPr/><a:p><a:r><a:t>不變</a:t></a:r></a:p></xdr:txBody></xdr:sp></xdr:grpSp><xdr:clientData/></xdr:twoCellAnchor></xdr:wsDr>`)
	sh := model.NewSheet("Sheet1")
	oldShape := &model.Shape{SourceID: 7, Font: model.DefaultFont, Text: &model.RichText{Runs: []model.TextRun{{Text: "原始文字"}}}}
	old := []model.Object{{ID: 1, Kind: model.ObjectShape, Shape: oldShape, ReadOnly: true}}
	next := *oldShape
	next.Text = &model.RichText{Runs: []model.TextRun{{Text: "修改\n\n中文🙂"}}}
	obj := old[0]
	obj.Shape = &next
	sh.Layout.Objects = []model.Object{obj}
	result, err := patchShapeDrawing(source, sh, old, nil)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := drawingNodes(source)
	b, _ := drawingNodes(result)
	if findShape(a, 8).raw(source) != findShape(b, 8).raw(result) {
		t.Fatal("edited group sibling")
	}
	for _, name := range []string{"bodyPr", "lstStyle", "pPr"} {
		if a.all(name)[0].raw(source) != b.all(name)[0].raw(result) {
			t.Fatalf("changed %s", name)
		}
	}
	before := findShape(a, 7).child("txBody")
	after := findShape(b, 7).child("txBody")
	if string(applyDrawingEdits(source, []drawingEdit{{before.start, before.end, ""}})) != string(applyDrawingEdits(result, []drawingEdit{{after.start, after.end, ""}})) {
		t.Fatal("changed drawing outside text body")
	}
	chart := model.Object{ID: 2, Kind: model.ObjectChart, Chart: &model.Chart{Type: "column"}}
	if len(RewrittenCharts(append(old, chart), append(sh.Layout.Objects, chart))) != 0 {
		t.Fatal("text edit requested chart rewrite")
	}
}

func TestShapeParagraphTemplatesSurviveSavedUndo(t *testing.T) {
	source := []byte(`<xdr:wsDr xmlns:xdr="` + spreadsheetDrawingNS + `" xmlns:a="` + drawingNS + `"><xdr:oneCellAnchor><xdr:sp><xdr:nvSpPr><xdr:cNvPr id="7"/></xdr:nvSpPr><xdr:spPr/><xdr:txBody><a:bodyPr/><a:p><a:pPr lvl="0"/><a:r><a:t>甲</a:t></a:r></a:p><a:p><a:pPr lvl="1"/><a:r><a:t>乙</a:t></a:r></a:p><a:p><a:pPr lvl="2"/><a:r><a:t>丙</a:t></a:r></a:p></xdr:txBody></xdr:sp></xdr:oneCellAnchor></xdr:wsDr>`)
	root, _ := drawingNodes(source)
	original := &model.Shape{SourceID: 7, RawXML: standaloneShape(source, root, findShape(root, 7)), Font: model.DefaultFont, Text: &model.RichText{Runs: []model.TextRun{{Text: "甲\n乙\n丙"}}}}
	edited := *original
	edited.Text = &model.RichText{Runs: []model.TextRun{{Text: "新"}}}
	saved := replaceShapeText(source, findShape(root, 7), &edited)
	parsed, _ := drawingNodes(saved)
	restored := replaceShapeText(saved, findShape(parsed, 7), original)
	after, err := drawingNodes(restored)
	if err != nil {
		t.Fatal(err)
	}
	levels := after.all("pPr")
	if len(levels) != 3 {
		t.Fatal("undo lost paragraphs")
	}
	for i, node := range levels {
		if node.attr("lvl") != strconv.Itoa(i) {
			t.Fatal("saved undo lost paragraph properties")
		}
	}
}
