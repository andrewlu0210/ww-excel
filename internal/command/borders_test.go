package command

import (
	"bytes"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

func borderAt(wb *model.Workbook, r, c int) model.Borders {
	return wb.Styles.Get(wb.Sheets[0].EffectiveStyle(r, c)).Border
}
func TestBorderStyleEdgesAndHistory(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	old := wb.Styles.Get(0)
	old.Fill = "ABCDEF"
	old.Border.Top = model.BorderLine{Style: "dotted", Color: "123456"}
	sh.Set(2, 2, model.Cell{S: wb.Styles.Intern(old)})
	red := model.BorderLine{Style: "double", Color: "FF0000"}
	blue := model.BorderLine{Style: "dashed", Color: "0000FF"}
	cmd, err := ApplyBorderStyle(wb, 0, []model.Range{{R0: 1, C0: 1, R1: 3, C1: 3}}, "框線", bold, BorderPatch{Left: &red, Right: &red, InsideH: &blue})
	if err != nil {
		t.Fatal(err)
	}
	if borderAt(wb, 1, 1).Left.Style != "" {
		t.Fatal("builder changed workbook")
	}
	h := NewHistory(0)
	h.Exec(wb, cmd)
	if borderAt(wb, 2, 1).Left != red || borderAt(wb, 2, 0).Right != red || borderAt(wb, 2, 3).Right != red || borderAt(wb, 2, 4).Left != red {
		t.Fatal("outside shared edges differ")
	}
	if borderAt(wb, 1, 2).Bottom != blue || borderAt(wb, 2, 2).Top != blue || borderAt(wb, 2, 2).Bottom != blue || borderAt(wb, 3, 2).Top != blue {
		t.Fatal("inside shared edges differ")
	}
	if borderAt(wb, 1, 2).Top.Style != "" || borderAt(wb, 3, 2).Bottom.Style != "" {
		t.Fatal("unmodified outer edges changed")
	}
	if st := wb.Styles.Get(sh.EffectiveStyle(2, 2)); st.Fill != "ABCDEF" || !st.Font.Bold {
		t.Fatal("other format lost")
	}
	h.Undo(wb)
	if sh.Get(2, 2).S != wb.Styles.Intern(old) || borderAt(wb, 2, 0).Right.Style != "" {
		t.Fatal("undo did not restore neighbors")
	}
	h.Redo(wb)
	if borderAt(wb, 2, 0).Right != red {
		t.Fatal("redo")
	}
	var data bytes.Buffer
	if err := wwx.Write(&data, wb); err != nil {
		t.Fatal(err)
	}
	restored, err := wwx.Read(&data)
	if err != nil {
		t.Fatal(err)
	}
	if borderAt(restored, 2, 0) != borderAt(wb, 2, 0) || borderAt(restored, 2, 2) != borderAt(wb, 2, 2) {
		t.Fatal("WWX roundtrip")
	}
}
func TestBorderClearOnlySpecifiedSharedEdge(t *testing.T) {
	wb := model.NewWorkbook()
	line := model.BorderLine{Style: "thick", Color: "123456"}
	empty := model.BorderLine{}
	cmd, _ := ApplyBorderStyle(wb, 0, []model.Range{{R0: 1, C0: 1, R1: 2, C1: 2}}, "框線", func(s model.Style, _ At) model.Style { return s }, BorderPatch{Top: &line, Bottom: &line, InsideV: &line})
	cmd.Do(wb)
	cmd, err := ApplyBorderStyle(wb, 0, []model.Range{{R0: 1, C0: 1, R1: 2, C1: 2}}, "框線", func(s model.Style, _ At) model.Style { return s }, BorderPatch{Top: &empty})
	if err != nil {
		t.Fatal(err)
	}
	cmd.Do(wb)
	if borderAt(wb, 1, 1).Top.Style != "" || borderAt(wb, 0, 1).Bottom.Style != "" {
		t.Fatal("opposing old line survived clear")
	}
	if borderAt(wb, 2, 1).Bottom != line || borderAt(wb, 1, 1).Right != line {
		t.Fatal("untouched edge changed")
	}
}
func TestBorderMergedRangeAndInside(t *testing.T) {
	wb := model.NewWorkbook()
	wb.Sheets[0].Layout.Merges = []model.Range{{R0: 1, C0: 1, R1: 2, C1: 2}}
	line := model.BorderLine{Style: "medium", Color: "123456"}
	inner := model.BorderLine{Style: "dotted", Color: "ABCDEF"}
	cmd, err := ApplyBorderStyle(wb, 0, []model.Range{{R0: 1, C0: 1, R1: 1, C1: 1}}, "框線", bold, BorderPatch{Top: &line, Bottom: &line, Left: &line, Right: &line, InsideH: &inner, InsideV: &inner})
	if err != nil {
		t.Fatal(err)
	}
	cmd.Do(wb)
	if b := borderAt(wb, 1, 1); b.Top != line || b.Bottom != line || b.Left != line || b.Right != line {
		t.Fatalf("anchor = %+v", b)
	}
	if borderAt(wb, 2, 2).Bottom != line || borderAt(wb, 2, 2).Right != line || borderAt(wb, 3, 1).Top != line {
		t.Fatal("merge perimeter")
	}
	if borderAt(wb, 2, 2).Top.Style != "" || borderAt(wb, 2, 2).Left.Style != "" {
		t.Fatal("inside line crosses merge")
	}
}
func TestBorderWholeColumnIsSparse(t *testing.T) {
	wb := model.NewWorkbook()
	line := model.BorderLine{Style: "thin", Color: "FF0000"}
	inner := model.BorderLine{Style: "dashed", Color: "0000FF"}
	cmd, err := ApplyBorderStyle(wb, 0, []model.Range{{R0: 0, C0: 2, R1: model.MaxRows - 1, C1: 3}}, "框線", func(s model.Style, _ At) model.Style { return s }, BorderPatch{Top: &line, Bottom: &line, Left: &line, InsideH: &inner, InsideV: &inner})
	if err != nil {
		t.Fatal(err)
	}
	cmd.Do(wb)
	if borderAt(wb, 0, 2).Top != line || borderAt(wb, model.MaxRows-1, 2).Bottom != line || borderAt(wb, 600000, 2).Top != inner || borderAt(wb, 600000, 2).Bottom != inner || borderAt(wb, 600000, 2).Right != inner || borderAt(wb, 600000, 1).Right != line {
		t.Fatal("whole-column edges")
	}
	if wb.Sheets[0].BlockCount() > 4 {
		t.Fatalf("allocated %d blocks", wb.Sheets[0].BlockCount())
	}
}

func TestBorderWholeRowsPreserveColumnStylesAndNeighbors(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	st := wb.Styles.Get(0)
	st.Fill = "ABCDEF"
	sh.Layout.ColStyles[4] = wb.Styles.Intern(st)
	line := model.BorderLine{Style: "double", Color: "123456"}
	inner := model.BorderLine{Style: "dotted"}
	cmd, err := ApplyBorderStyle(wb, 0, []model.Range{{R0: 2, C0: 0, R1: 3, C1: model.MaxCols - 1}}, "框線", func(s model.Style, _ At) model.Style { return s }, BorderPatch{Top: &line, Bottom: &line, Left: &line, Right: &line, InsideH: &inner, InsideV: &inner})
	if err != nil {
		t.Fatal(err)
	}
	cmd.Do(wb)
	if borderAt(wb, 2, 0).Left != line || borderAt(wb, 3, model.MaxCols-1).Right != line || borderAt(wb, 2, 8000).Right != inner || borderAt(wb, 2, 8000).Bottom != inner || borderAt(wb, 1, 8000).Bottom != line {
		t.Fatal("whole-row edges")
	}
	if wb.Styles.Get(sh.EffectiveStyle(2, 4)).Fill != "ABCDEF" || wb.Styles.Get(sh.EffectiveStyle(1, 4)).Fill != "ABCDEF" || wb.Styles.Get(sh.EffectiveStyle(4, 4)).Fill != "ABCDEF" {
		t.Fatal("neighbor column fill lost")
	}
	if sh.BlockCount() > 6 {
		t.Fatal("whole row materialized empty cells")
	}
}
func TestBorderDisjointAndMergedNeighbor(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	sh.Layout.Merges = []model.Range{{R0: 0, C0: 1, R1: 0, C1: 4}}
	line := model.BorderLine{Style: "thick", Color: "FF0000"}
	cmd, err := ApplyBorderStyle(wb, 0, []model.Range{{R0: 1, C0: 2, R1: 2, C1: 3}, {R0: 5, C0: 5, R1: 6, C1: 6}}, "框線", bold, BorderPatch{Top: &line, Bottom: &line})
	if err != nil {
		t.Fatal(err)
	}
	cmd.Do(wb)
	if borderAt(wb, 0, 1).Bottom != line || borderAt(wb, 0, 4).Bottom != line || borderAt(wb, 5, 5).Top != line {
		t.Fatal("disjoint or merged neighbor border")
	}
	if wb.Styles.Get(sh.EffectiveStyle(0, 1)).Font.Bold {
		t.Fatal("neighbor inherited unrelated font edit")
	}
	if borderAt(wb, 4, 4) != (model.Borders{}) {
		t.Fatal("gap changed")
	}
}
func TestBorderLimitDoesNotMutateWorkbook(t *testing.T) {
	wb := model.NewWorkbook()
	line := model.BorderLine{Style: "thin"}
	cmd, err := ApplyBorderStyle(wb, 0, []model.Range{{R0: 0, C0: 0, R1: 1024, C1: 1024}}, "框線", bold, BorderPatch{Top: &line})
	if err != ErrFillTooLarge || cmd != nil {
		t.Fatalf("cmd=%v err=%v", cmd, err)
	}
	if wb.Sheets[0].BlockCount() != 0 || wb.Styles.Get(0).Font.Bold {
		t.Fatal("rejected plan changed document")
	}
}

func TestBorderClearOverridesInheritedStyle(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	st := wb.Styles.Get(0)
	st.Border.Left = model.BorderLine{Style: "thin"}
	sh.Layout.ColStyles[2] = wb.Styles.Intern(st)
	empty := model.BorderLine{}
	cmd, err := ApplyBorderStyle(wb, 0, []model.Range{{R0: 4, C0: 2, R1: 4, C1: 2}}, "框線", func(s model.Style, _ At) model.Style { return s }, BorderPatch{Left: &empty})
	if err != nil {
		t.Fatal(err)
	}
	cmd.Do(wb)
	if borderAt(wb, 4, 2).Left.Style != "" || sh.Get(4, 2).S == 0 {
		t.Fatal("clearing a border must override the inherited column style")
	}
	if borderAt(wb, 5, 2).Left.Style != "thin" {
		t.Fatal("other inherited cells changed")
	}
	var data bytes.Buffer
	if err := wwx.Write(&data, wb); err != nil {
		t.Fatal(err)
	}
	restored, err := wwx.Read(&data)
	if err != nil {
		t.Fatal(err)
	}
	if borderAt(restored, 4, 2).Left.Style != "" {
		t.Fatal("explicit clear lost on WWX reopen")
	}
}
