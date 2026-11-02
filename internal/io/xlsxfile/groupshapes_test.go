package xlsxfile

import (
	"encoding/xml"
	"testing"

	"ww-excel/internal/model"
)

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
