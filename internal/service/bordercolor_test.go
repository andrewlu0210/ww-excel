package service

import (
	"testing"
	"ww-excel/internal/model"
)

func TestRecolorBordersPreservesStylesAndSynchronizesNeighbors(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	sh.Set(3, 3, model.StringCell("unchanged"))
	red := "FF0000"
	old := "0070C0"
	fill := "FFF2CC"
	on := true
	if err := svc.Format(bg, 0, []RangeDTO{{3, 3, 4, 4}}, FormatPatch{Fill: &fill, Diagonal: &BorderLineDTO{Style: "dashed", Color: old}, DiagonalUp: &on, Borders: &BorderPatchDTO{Top: &BorderLineDTO{Style: "double", Color: old}, Left: &BorderLineDTO{Style: "thick", Color: old}, InsideH: &BorderLineDTO{Style: "dotted", Color: old}, InsideV: &BorderLineDTO{Style: "medium", Color: old}}}); err != nil {
		t.Fatal(err)
	}
	before := map[[2]int]model.Style{}
	for r := 2; r <= 5; r++ {
		for c := 2; c <= 5; c++ {
			before[[2]int{r, c}] = wb.Styles.Get(sh.EffectiveStyle(r, c))
		}
	}
	if err := svc.Format(bg, 0, []RangeDTO{{3, 3, 4, 4}}, FormatPatch{RecolorBorders: &red}); err != nil {
		t.Fatal(err)
	}
	at := func(r, c int) model.Style { return wb.Styles.Get(sh.EffectiveStyle(r, c)) }
	for r := 3; r <= 4; r++ {
		for c := 3; c <= 4; c++ {
			st := at(r, c)
			want := before[[2]int{r, c}]
			for _, line := range []*model.BorderLine{&want.Border.Top, &want.Border.Bottom, &want.Border.Left, &want.Border.Right, &want.Border.Diagonal} {
				if line.Style != "" {
					line.Color = red
				}
			}
			if st != want {
				t.Fatalf("(%d,%d) %+v want %+v", r, c, st, want)
			}
		}
	}
	if at(2, 3).Border.Bottom.Color != red || at(3, 2).Border.Right.Color != red || at(5, 3) != before[[2]int{5, 3}] {
		t.Fatal("neighbor recolor mismatch")
	}
	if sh.Get(3, 3).Str != "unchanged" {
		t.Fatal("content changed")
	}
	svc.Undo(bg)
	for rc, want := range before {
		if at(rc[0], rc[1]) != want {
			t.Fatalf("undo %v", rc)
		}
	}
	svc.Redo(bg)
	if at(3, 3).Border.Top.Color != red {
		t.Fatal("redo")
	}
	auto := ""
	if err := svc.Format(bg, 0, []RangeDTO{{3, 3, 4, 4}}, FormatPatch{RecolorBorders: &auto}); err != nil {
		t.Fatal(err)
	}
	if at(3, 3).Border.Top.Style != "double" || at(3, 3).Border.Top.Color != "" || at(2, 3).Border.Bottom.Color != "" {
		t.Fatal("automatic color must preserve strokes")
	}
	bad := "invalid"
	if err := svc.Format(bg, 0, []RangeDTO{{}}, FormatPatch{RecolorBorders: &bad}); err == nil {
		t.Fatal("invalid color accepted")
	}
}
func TestRecolorMergedAndSparseWholeColumn(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	sh.Layout.Merges = []model.Range{{R0: 3, C0: 3, R1: 4, C1: 4}}
	if err := svc.Format(bg, 0, []RangeDTO{{3, 3, 4, 4}}, FormatPatch{Border: "outside"}); err != nil {
		t.Fatal(err)
	}
	color := "00B050"
	if err := svc.Format(bg, 0, []RangeDTO{{4, 4, 4, 4}}, FormatPatch{RecolorBorders: &color}); err != nil {
		t.Fatal(err)
	}
	at := func(r, c int) model.Style { return wb.Styles.Get(sh.EffectiveStyle(r, c)) }
	if at(3, 3).Border.Bottom.Color != color || at(5, 4).Border.Top.Color != color || at(4, 5).Border.Left.Color != color {
		t.Fatal("merged perimeter color")
	}
	if err := svc.Format(bg, 0, []RangeDTO{{0, 8, model.MaxRows - 1, 8}}, FormatPatch{Border: "all"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Format(bg, 0, []RangeDTO{{0, 8, model.MaxRows - 1, 8}}, FormatPatch{RecolorBorders: &color}); err != nil {
		t.Fatal(err)
	}
	lineStyle := "double"
	if err := svc.Format(bg, 0, []RangeDTO{{0, 8, model.MaxRows - 1, 8}}, FormatPatch{RestyleBorders: &lineStyle}); err != nil {
		t.Fatal(err)
	}
	if at(model.MaxRows-1, 8).Border.Right.Style != "double" || at(900000, 9).Border.Left.Style != "double" {
		t.Fatal("sparse whole-column restyle")
	}
	if at(model.MaxRows-1, 8).Border.Right.Color != color || at(900000, 9).Border.Left.Color != color || sh.BlockCount() > 16 {
		t.Fatal("whole-column recolor must stay sparse")
	}
}

func TestRestyleBordersPreservesColorsAndMissingEdges(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	if err := svc.Format(bg, 0, []RangeDTO{{3, 3, 3, 3}}, FormatPatch{Borders: &BorderPatchDTO{Top: &BorderLineDTO{Style: "thin", Color: "FF0000"}, Bottom: &BorderLineDTO{Style: "dotted", Color: "0070C0"}}}); err != nil {
		t.Fatal(err)
	}
	style := "double"
	if err := svc.Format(bg, 0, []RangeDTO{{3, 3, 3, 3}}, FormatPatch{RestyleBorders: &style}); err != nil {
		t.Fatal(err)
	}
	at := func(r, c int) model.Style { return wb.Styles.Get(sh.EffectiveStyle(r, c)) }
	st := at(3, 3)
	if st.Border.Top.Style != "double" || st.Border.Top.Color != "FF0000" || st.Border.Bottom.Style != "double" || st.Border.Bottom.Color != "0070C0" || st.Border.Left.Style != "" || at(2, 3).Border.Bottom.Style != "double" || at(4, 3).Border.Top.Style != "double" {
		t.Fatal("restyle must keep colors and missing sides", st)
	}
	svc.Undo(bg)
	if at(3, 3).Border.Top.Style != "thin" || at(3, 3).Border.Bottom.Style != "dotted" {
		t.Fatal("restyle undo")
	}
	svc.Redo(bg)
	if at(3, 3).Border.Top.Style != "double" {
		t.Fatal("restyle redo")
	}
	for _, bad := range []string{"", "invalid"} {
		if err := svc.Format(bg, 0, []RangeDTO{{3, 3, 3, 3}}, FormatPatch{RestyleBorders: &bad}); err == nil {
			t.Fatal("invalid style accepted")
		}
	}
}
