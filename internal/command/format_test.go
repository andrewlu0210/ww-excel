package command

import (
	"testing"

	"ww-excel/internal/model"
)

func bold(s model.Style, _ At) model.Style { s.Font.Bold = true; return s }

func TestApplyStyleCells(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	sh.Set(0, 0, model.NumberCell(1))
	h := NewHistory(0)

	cmd, err := ApplyStyle(wb, 0, []model.Range{{R0: 0, C0: 0, R1: 1, C1: 1}}, "粗體", bold)
	if err != nil {
		t.Fatal(err)
	}
	h.Exec(wb, cmd)
	for _, rc := range [][2]int{{0, 0}, {1, 1}} {
		if !wb.Styles.Get(sh.Get(rc[0], rc[1]).S).Font.Bold {
			t.Fatalf("cell %v not bold", rc)
		}
	}
	if sh.Get(0, 0).Num != 1 {
		t.Fatal("value lost")
	}
	// Identical styles share an ID.
	if sh.Get(0, 0).S != sh.Get(1, 1).S {
		t.Fatal("style not interned")
	}
	h.Undo(wb)
	if sh.Get(0, 0).S != 0 || !sh.Get(1, 1).IsEmpty() {
		t.Fatal("undo did not restore")
	}
}

func TestApplyStyleWholeColumn(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	sh.Set(900_000, 2, model.StringCell("x"))
	h := NewHistory(0)
	col := model.Range{R0: 0, C0: 2, R1: model.MaxRows - 1, C1: 2}
	cmd, err := ApplyStyle(wb, 0, []model.Range{col}, "填滿", func(s model.Style, _ At) model.Style { s.Fill = "FFFF00"; return s })
	if err != nil {
		t.Fatal(err)
	}
	eff := h.Exec(wb, cmd)
	if !eff.LayoutChanged {
		t.Fatal("column style should change the layout")
	}
	if wb.Styles.Get(sh.Layout.ColStyles[2]).Fill != "FFFF00" {
		t.Fatal("column style not set")
	}
	if wb.Styles.Get(sh.EffectiveStyle(5, 2)).Fill != "FFFF00" {
		t.Fatal("empty cell should inherit the column style")
	}
	if wb.Styles.Get(sh.Get(900_000, 2).S).Fill != "FFFF00" {
		t.Fatal("existing cell not restyled")
	}
	if sh.BlockCount() != 1 {
		t.Fatalf("whole-column format allocated %d blocks", sh.BlockCount())
	}
	h.Undo(wb)
	if len(sh.Layout.ColStyles) != 0 {
		t.Fatal("undo did not restore the layout")
	}
}

func TestSetCellKeepsAndDetectsFormat(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	h := NewHistory(0)
	cmd, _ := ApplyStyle(wb, 0, []model.Range{{R0: 0, C0: 0, R1: 0, C1: 0}}, "粗體", bold)
	h.Exec(wb, cmd)

	h.Exec(wb, SetCell(wb, 0, 0, 0, model.NumberCell(0.12), "0%", false))
	st := wb.Styles.Get(sh.Get(0, 0).S)
	if !st.Font.Bold || st.NumFmt != "0%" {
		t.Fatalf("style after typing 12%% = %+v", st)
	}
	// An existing number format is not replaced by detection.
	h.Exec(wb, SetCell(wb, 0, 0, 0, model.NumberCell(5), "yyyy/m/d", false))
	if wb.Styles.Get(sh.Get(0, 0).S).NumFmt != "0%" {
		t.Fatal("detected format replaced an explicit one")
	}
	// Delete keeps formats.
	h.Exec(wb, ClearRanges(wb, 0, []model.Range{{R0: 0, C0: 0, R1: 0, C1: 0}}, ClearContents))
	if c := sh.Get(0, 0); c.HasContent() || !wb.Styles.Get(c.S).Font.Bold {
		t.Fatalf("after Delete: %+v", c)
	}
	h.Exec(wb, ClearRanges(wb, 0, []model.Range{{R0: 0, C0: 0, R1: 0, C1: 0}}, ClearFormats))
	if !sh.Get(0, 0).IsEmpty() {
		t.Fatal("clear formats left a style")
	}
}

func TestMerge(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	sh.Set(0, 0, model.StringCell("title"))
	sh.Set(0, 1, model.StringCell("gone"))
	h := NewHistory(0)

	cmd, err := Merge(wb, 0, model.Range{R0: 0, C0: 0, R1: 0, C1: 2}, true)
	if err != nil {
		t.Fatal(err)
	}
	h.Exec(wb, cmd)
	if len(sh.Layout.Merges) != 1 || sh.Get(0, 1).HasContent() {
		t.Fatalf("merge = %+v, B1 = %+v", sh.Layout.Merges, sh.Get(0, 1))
	}
	if wb.Styles.Get(sh.Get(0, 0).S).Align.H != "center" {
		t.Fatal("merge & center should center")
	}
	if _, err := Merge(wb, 0, model.Range{R0: 0, C0: 2, R1: 1, C1: 3}, false); err != ErrMergeOverlap {
		t.Fatalf("partial overlap err = %v", err)
	}
	// A larger merge absorbs the existing one.
	cmd, err = Merge(wb, 0, model.Range{R0: 0, C0: 0, R1: 2, C1: 3}, false)
	if err != nil {
		t.Fatal(err)
	}
	h.Exec(wb, cmd)
	if len(sh.Layout.Merges) != 1 || sh.Layout.Merges[0].C1 != 3 {
		t.Fatalf("merges = %+v", sh.Layout.Merges)
	}
	h.Exec(wb, Unmerge(wb, 0, []model.Range{{R0: 1, C0: 1, R1: 1, C1: 1}}))
	if len(sh.Layout.Merges) != 0 {
		t.Fatal("unmerge")
	}
	h.Undo(wb)
	h.Undo(wb)
	h.Undo(wb)
	if len(sh.Layout.Merges) != 0 || sh.Get(0, 1).Str != "gone" {
		t.Fatal("undo all merges")
	}
}

func TestSetSizes(t *testing.T) {
	wb := model.NewWorkbook()
	sh := wb.Sheets[0]
	h := NewHistory(0)
	h.Exec(wb, SetSizes(wb, 0, false, []int{1, 2}, 120))
	if sh.Layout.ColWidth(2) != 120 || sh.Layout.ColWidth(3) != model.DefaultColWidth {
		t.Fatal("column widths")
	}
	h.Exec(wb, SetSizes(wb, 0, true, []int{5}, 40))
	if sh.Layout.RowHeight(5) != 40 {
		t.Fatal("row height")
	}
	h.Undo(wb)
	h.Undo(wb)
	if len(sh.Layout.Cols) != 0 || len(sh.Layout.Rows) != 0 {
		t.Fatal("undo sizes")
	}
}
