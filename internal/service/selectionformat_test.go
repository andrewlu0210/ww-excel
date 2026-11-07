package service

import (
	"context"
	"slices"
	"testing"
	"ww-excel/internal/model"
)

func TestSelectionFormatMixedAndSparseInheritance(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	a := wb.Styles.Get(0)
	a.Fill = "FF0000"
	a.Font.Bold = true
	a.NumFmt = "0.00"
	b := a
	b.Fill = "0000FF"
	b.Font.Name = "Arial"
	b.Font.Bold = false
	b.NumFmt = "0%"
	sh.Set(0, 0, model.Cell{S: wb.Styles.Intern(a)})
	sh.Set(0, 1, model.Cell{S: wb.Styles.Intern(b)})
	summary, err := svc.SelectionFormat(bg, 0, []RangeDTO{{0, 0, 0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"fill", "bold", "fontName", "numFmt"} {
		if !slices.Contains(summary.Mixed, key) {
			t.Fatalf("missing mixed %s: %v", key, summary.Mixed)
		}
	}
	if slices.Contains(summary.Mixed, "fontSize") {
		t.Fatal("shared font size marked mixed")
	}
	sh.Layout.ColStyles[2] = wb.Styles.Intern(a)
	summary, err = svc.SelectionFormat(bg, 0, []RangeDTO{{0, 2, model.MaxRows - 1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Style.Fill != "FF0000" || len(summary.Mixed) != 0 {
		t.Fatalf("sparse column summary=%+v", summary)
	}
	sh.Set(900000, 2, model.Cell{S: wb.Styles.Intern(b)})
	summary, err = svc.SelectionFormat(bg, 0, []RangeDTO{{0, 2, model.MaxRows - 1, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(summary.Mixed, "fill") {
		t.Fatal("sparse explicit style not aggregated")
	}
	if len(rec.events) != 0 || sess.doc.Dirty() {
		t.Fatal("summary modified document")
	}
}
func TestSelectionFormatRichTextAndMergedBorders(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	bold := true
	off := false
	c := model.StringCell("AB")
	c.Rich = &model.RichText{Runs: []model.TextRun{{Text: "A", Font: &model.RichFont{Bold: &bold}}, {Text: "B", Font: &model.RichFont{Bold: &off}}}}
	sh.Set(0, 0, c)
	summary, err := svc.SelectionFormat(bg, 0, []RangeDTO{{}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(summary.Mixed, "bold") {
		t.Fatal("rich runs must be mixed")
	}
	sh.Layout.Merges = []model.Range{{R0: 2, C0: 2, R1: 3, C1: 3}}
	line := &BorderLineDTO{Style: "double", Color: "123456"}
	if err = svc.Format(bg, 0, []RangeDTO{{2, 2, 3, 3}}, FormatPatch{Borders: &BorderPatchDTO{Top: line, Bottom: line, Left: line, Right: line}}); err != nil {
		t.Fatal(err)
	}
	summary, err = svc.SelectionFormat(bg, 0, []RangeDTO{{2, 2, 3, 3}})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Borders.Bottom == nil || summary.Borders.Bottom.Style != "double" || slices.Contains(summary.Mixed, "bottom") {
		t.Fatalf("merged border summary=%+v", summary)
	}
}
func TestFormatNoopAndAtomicAutomaticRow(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	sh.Set(0, 0, model.StringCell("large"))
	off := false
	if err := svc.Format(bg, 0, []RangeDTO{{}}, FormatPatch{Bold: &off}); err != nil {
		t.Fatal(err)
	}
	if sess.doc.Dirty() || len(rec.events) != 0 {
		t.Fatal("no-op must not add undo or events")
	}
	size := 36.0
	fill := "FFFF00"
	if err := svc.Format(bg, 0, []RangeDTO{{}}, FormatPatch{FontSize: &size, Fill: &fill, AutoRows: []SizeDTO{{I: 0, Px: 64}}}); err != nil {
		t.Fatal(err)
	}
	if wb.Styles.Get(sh.EffectiveStyle(0, 0)).Fill != fill || sh.Layout.RowHeight(0) != 64 {
		t.Fatal("combined format not applied")
	}
	svc.Undo(bg)
	if sh.Layout.RowHeight(0) != 20 || wb.Styles.Get(sh.EffectiveStyle(0, 0)).Font.Size != 11 || sess.State().CanUndo {
		t.Fatal("one undo must restore font, fill and row height")
	}
	svc.Redo(bg)
	if sh.Layout.RowHeight(0) != 64 {
		t.Fatal("redo height")
	}
	sh.Layout.Rows[1] = model.Size{Px: 23}
	sh.Set(1, 0, model.StringCell("manual"))
	if err := svc.Format(bg, 0, []RangeDTO{{1, 0, 1, 0}}, FormatPatch{FontSize: &size, AutoRows: []SizeDTO{{I: 1, Px: 80}}}); err != nil {
		t.Fatal(err)
	}
	if sh.Layout.RowHeight(1) != 23 {
		t.Fatal("manual row height overwritten")
	}
}

func TestSelectionFormatIncludesBlankAndDisjointCells(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	st := wb.Styles.Get(0)
	st.Fill = "FF0000"
	sh.Set(0, 0, model.Cell{S: wb.Styles.Intern(st)})
	summary, err := svc.SelectionFormat(bg, 0, []RangeDTO{{0, 0, 0, 0}, {8, 8, 8, 8}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(summary.Mixed, "fill") {
		t.Fatal("blank cells must be included")
	}
	if sh.BlockCount() != 1 {
		t.Fatal("summary materialized blanks")
	}
}
func TestFormatRowsProjectsRichFontsAndPreservesManualHeights(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	sh.Set(2, 0, model.StringCell("new font"))
	sh.Set(2, 1, model.StringCell("outside"))
	sh.Set(3, 0, model.StringCell("manual"))
	sh.Layout.Rows[3] = model.Size{Px: 25}
	big := 50.0
	c := model.StringCell("rich")
	c.Rich = &model.RichText{Runs: []model.TextRun{{Text: "rich", Font: &model.RichFont{Size: &big}}}}
	sh.Set(2, 2, c)
	size := 36.0
	rows, err := svc.FormatRows(bg, 0, []RangeDTO{{2, 0, 3, 0}}, FormatPatch{FontSize: &size})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Row != 2 || len(rows[0].Cells) != 3 {
		t.Fatalf("rows=%+v", rows)
	}
	if rows[0].Cells[0].Style.Font.Size != 36 || rows[0].Cells[1].Style.Font.Size != 11 || rows[0].Cells[2].Runs[0].Font.Size != 50 {
		t.Fatal("projection lost selected or unselected fonts")
	}
	if wb.Styles.Get(sh.EffectiveStyle(2, 0)).Font.Size != 11 || len(rec.events) != 0 || sess.doc.Dirty() {
		t.Fatal("projection modified document")
	}
}
func TestFormatNoopWholeAxesAndBorders(t *testing.T) {
	for _, rg := range []RangeDTO{{0, 2, model.MaxRows - 1, 2}, {2, 0, 2, model.MaxCols - 1}} {
		sess, rec := newTestSession(false)
		svc := NewSheetService(sess)
		empty := ""
		if err := svc.Format(bg, 0, []RangeDTO{rg}, FormatPatch{Fill: &empty, Border: "none"}); err != nil {
			t.Fatal(err)
		}
		if sess.doc.Dirty() || len(rec.events) != 0 {
			t.Fatal("whole-axis no-op added history")
		}
	}
}

func TestSelectionFormatWholeSheetAndCancellation(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	st := wb.Styles.Get(0)
	st.Fill = "FF0000"
	sh.Layout.ColStyles[2] = wb.Styles.Intern(st)
	st.Font.Bold = true
	sh.Set(900000, 12000, model.Cell{S: wb.Styles.Intern(st)})
	summary, err := svc.SelectionFormat(bg, 0, []RangeDTO{{0, 0, model.MaxRows - 1, model.MaxCols - 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(summary.Mixed, "fill") || !slices.Contains(summary.Mixed, "bold") || sh.BlockCount() != 1 {
		t.Fatal("whole-sheet sparse summary")
	}
	ctx, cancel := context.WithCancel(bg)
	cancel()
	if _, err := svc.SelectionFormat(ctx, 0, []RangeDTO{{}}); err != context.Canceled {
		t.Fatal("summary cancellation")
	}
	if _, err := svc.FormatRows(ctx, 0, []RangeDTO{{}}, FormatPatch{}); err != context.Canceled {
		t.Fatal("projection cancellation")
	}
}
func TestFormatNoopPreservesInheritedOverrides(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	st := wb.Styles.Get(0)
	st.Fill = "FF0000"
	st.Font.Name = "Arial"
	sh.Layout.ColStyles[2] = wb.Styles.Intern(st)
	empty := ""
	// A clear changes this column intersection but does not flatten its font.
	if err := svc.Format(bg, 0, []RangeDTO{{2, 0, 2, model.MaxCols - 1}}, FormatPatch{Fill: &empty}); err != nil {
		t.Fatal(err)
	}
	got := wb.Styles.Get(sh.EffectiveStyle(2, 2))
	if got.Font.Name != "Arial" || got.Fill != "" {
		t.Fatal("inheritance override lost")
	}
	before := len(rec.events)
	version := sess.doc.Version()
	if err := svc.Format(bg, 0, []RangeDTO{{2, 0, 2, model.MaxCols - 1}}, FormatPatch{Fill: &empty}); err != nil {
		t.Fatal(err)
	}
	if sess.doc.Version() != version || len(rec.events) != before {
		t.Fatal("repeated clear created history")
	}
}
