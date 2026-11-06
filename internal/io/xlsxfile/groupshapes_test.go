package xlsxfile

import (
	"encoding/xml"
	"testing"

	"ww-excel/internal/model"
)

func TestShapeVerticalOverflowClip(t *testing.T) {
	for _, mode := range []string{"clip", "overflow", ""} {
		var raw drawingShape
		if err := xml.Unmarshal([]byte(`<sp><spPr><prstGeom prst="roundRect"/></spPr><txBody><bodyPr vertOverflow="`+mode+`" anchor="ctr"/><p><r><t>Title</t></r></p></txBody></sp>`), &raw); err != nil {
			t.Fatal(err)
		}
		shape := raw.model(nil, 356, 76.8)
		if shape.ClipText != (mode == "clip") || shape.AlignV != "center" {
			t.Fatalf("incorrect overflow mode %q: %+v", mode, shape)
		}
	}
}

func TestGroupChildrenAreFlattenedIntoPixelFrame(t *testing.T) {
	const src = `<grpSp><grpSpPr><xfrm><off x="0" y="0"/><ext cx="1905000" cy="952500"/><chOff x="100" y="200"/><chExt cx="3810000" cy="1905000"/></xfrm></grpSpPr>
<sp><spPr><xfrm><off x="1905100" y="200"/><ext cx="1905000" cy="952500"/></xfrm><prstGeom prst="ellipse"/><solidFill><srgbClr val="FF0000"/></solidFill></spPr></sp></grpSp>`
	var g rawGroup
	if err := xml.Unmarshal([]byte(src), &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Items) != 1 || g.Items[0].Shape == nil {
		t.Fatalf("expected one shape child, got %+v", g.Items)
	}
	// The group frame is 200x100 px while child space is twice as large.
	r := g.childRect(g.Items[0].Shape.Properties.Transform, pxRect{0, 0, 200, 100})
	if r.x != 100 || r.y != 0 || r.w != 100 || r.h != 50 {
		t.Fatalf("unexpected child rect %+v", r)
	}
}

func TestShiftAnchorCrossesCells(t *testing.T) {
	l := model.NewLayout()
	a := shiftAnchor(&l, model.Anchor{Row: 1, Col: 1, DX: 5, DY: 5}, l.ColWidth(1)+10, l.RowHeight(1)*2+3)
	if a.Col != 2 || a.DX != 15 || a.Row != 3 || a.DY != 8 {
		t.Fatalf("unexpected anchor %+v", a)
	}
	b := shiftAnchor(&l, model.Anchor{Row: 2, Col: 2}, -10, -4)
	if b.Col != 1 || b.Row != 1 || b.DX != l.ColWidth(1)-10 || b.DY != l.RowHeight(1)-4 {
		t.Fatalf("unexpected anchor %+v", b)
	}
}

func TestShapePreservesExplicitLineBreaksAndFields(t *testing.T) {
	const src = `<sp><spPr><prstGeom prst="rect"/></spPr><txBody><bodyPr/><p><pPr algn="ctr"/><r><rPr sz="1100"/><t>水平座標軸</t></r><br><rPr sz="1100"/></br><r><rPr sz="1100"/><t>(類別座標軸)</t></r><br/><br/><fld><rPr sz="1100"/><t>2012</t></fld><endParaRPr/></p><p><r><t>next paragraph</t></r></p></txBody></sp>`
	var raw drawingShape
	if err := xml.Unmarshal([]byte(src), &raw); err != nil {
		t.Fatal(err)
	}
	shape := raw.model(nil, 200, 100)
	if shape == nil {
		t.Fatal("missing shape")
	}
	var text string
	for _, run := range shape.Text.Runs {
		text += run.Text
	}
	if text != "水平座標軸\n(類別座標軸)\n\n2012\nnext paragraph" || shape.AlignH != "center" {
		t.Fatalf("shape text: %q (%s)", text, shape.AlignH)
	}
}

func TestShapePresetWhiteAndBlankParagraphFonts(t *testing.T) {
	const src = `<sp><spPr><prstGeom prst="rect"/></spPr><style><fontRef idx="minor"><schemeClr val="lt1"/></fontRef></style><txBody><bodyPr/><p><r><rPr sz="1700"/><t>Heading</t></r></p><p><endParaRPr sz="1800"/></p><p><r><rPr sz="1400"/><t>Body</t></r></p><p><endParaRPr sz="1400"/></p><p><r><rPr sz="1700" b="1" u="sng"><solidFill><prstClr val="white"/></solidFill></rPr><t>深入了解</t></r></p></txBody></sp>`
	var raw drawingShape
	if err := xml.Unmarshal([]byte(src), &raw); err != nil {
		t.Fatal(err)
	}
	shape := raw.model(map[string]string{"lt1": "FFFFFF"}, 260, 260)
	if shape == nil || shape.Font.Color != "FFFFFF" {
		t.Fatalf("missing font reference: %+v", shape)
	}
	var blank18, whiteLink bool
	for _, r := range shape.Text.Runs {
		font := r.Font.Resolve(shape.Font)
		if r.Text == "\n" && font.Size == 18 {
			blank18 = true
		}
		if r.Text == "深入了解" && font.Color == "FFFFFF" && font.Bold && font.Underline {
			whiteLink = true
		}
	}
	if !blank18 || !whiteLink {
		t.Fatalf("lost blank font or white link: %+v", shape.Text.Runs)
	}
	var fill drawingFill
	if err := xml.Unmarshal([]byte(`<solidFill><prstClr val="white"><shade val="50000"/></prstClr></solidFill>`), &fill); err != nil {
		t.Fatal(err)
	}
	if color := fill.color(nil); color != "BCBCBC" {
		t.Fatalf("preset color transform: %s", color)
	}
}
