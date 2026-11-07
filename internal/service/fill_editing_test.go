package service

import (
	"bytes"
	"path/filepath"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
)

func TestFillDirectTableAndConditionalPriority(t *testing.T) {
	for _, tableStyle := range []string{"TableStyleMedium2", "Custom"} {
		t.Run(tableStyle, func(t *testing.T) {
			sess, _ := newTestSession(false)
			svc := NewSheetService(sess)
			wb := sess.doc.Workbook()
			sh := wb.Sheets[0]
			band := "D9E1F2"
			sh.Layout.Tables = []model.Table{{Name: "Sales", Style: tableStyle, Range: model.Range{R0: 0, C0: 0, R1: 4, C1: 2}, Appearance: &model.TableAppearance{FirstStripeFill: &band}}}
			white := "FFFFFF"
			rg := RangeDTO{1, 0, 1, 0}
			if err := svc.Format(bg, 0, []RangeDTO{rg}, FormatPatch{Fill: &white}); err != nil {
				t.Fatal(err)
			}
			displayed := func() string {
				look := newLook(sess.doc, 0)
				id, _, _, _, _ := look.style(1, 0, sh.Get(1, 0), sh.EffectiveStyle(1, 0))
				return wb.Styles.Get(id).Fill
			}
			if got := displayed(); got != white {
				t.Fatalf("direct white lost to table: %s", got)
			}
			sh.Layout.CondFormats = []model.CFRule{{Type: "formula", Ranges: []model.Range{{R0: 1, C0: 0, R1: 1, C1: 0}}, Value1: "TRUE", Format: model.CFFormat{Fill: band}}}
			if got := displayed(); got != band {
				t.Fatalf("conditional fill lost priority: %s", got)
			}
			if raw := wb.Styles.Get(sh.EffectiveStyle(1, 0)).Fill; raw != white {
				t.Fatalf("overlay overwrote raw fill: %s", raw)
			}
			sh.Layout.CondFormats = nil
			empty := ""
			if err := svc.Format(bg, 0, []RangeDTO{rg}, FormatPatch{Fill: &empty}); err != nil {
				t.Fatal(err)
			}
			if got := displayed(); got != band {
				t.Fatalf("clearing fill must reveal table band: %s", got)
			}
		})
	}
}

func TestFillClearInheritedAndRoundTrip(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	st := wb.Styles.Get(0)
	st.Fill = "112233"
	sh.Layout.ColStyles[2] = wb.Styles.Intern(st)
	empty := ""
	white := "FFFFFF"
	fill := func(book *model.Workbook, r, c int) string {
		return book.Styles.Get(book.Sheets[0].EffectiveStyle(r, c)).Fill
	}
	for _, value := range []string{white, empty} {
		if err := svc.Format(bg, 0, []RangeDTO{{3, 2, 3, 2}}, FormatPatch{Fill: &value}); err != nil {
			t.Fatal(err)
		}
		if got := fill(wb, 3, 2); got != value {
			t.Fatalf("fill=%q want=%q", got, value)
		}
	}
	if got := fill(wb, 4, 2); got != "112233" {
		t.Fatal("other inherited cell changed")
	}
	svc.Undo(bg)
	if fill(wb, 3, 2) != white {
		t.Fatal("undo no-fill")
	}
	svc.Redo(bg)
	if fill(wb, 3, 2) != "" {
		t.Fatal("redo no-fill")
	}
	if err := svc.Format(bg, 0, []RangeDTO{{3, 3, 3, 3}}, FormatPatch{Fill: &white}); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := wwx.Write(&data, wb); err != nil {
		t.Fatal(err)
	}
	w, err := wwx.Read(&data)
	if err != nil {
		t.Fatal(err)
	}
	for _, book := range []*model.Workbook{wb, w} {
		if fill(book, 3, 2) != "" || fill(book, 3, 3) != white || fill(book, 4, 2) != "112233" {
			t.Fatal("WWX fill semantics lost")
		}
	}
	path := filepath.Join(t.TempDir(), "fill.xlsx")
	f, err := xlsxfile.Save(path, wb, nil, xlsxfile.Changes{})
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	x, src, err := xlsxfile.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if fill(x, 3, 2) != "" || fill(x, 3, 3) != white || fill(x, 4, 2) != "112233" {
		t.Fatal("XLSX fill semantics lost")
	}
}

func TestFillCellStyleExcludesDisplayOverlays(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	st := wb.Styles.Get(0)
	st.Fill = "FFFFFF"
	sh.Layout.ColStyles[1] = wb.Styles.Intern(st)
	sh.Layout.CondFormats = []model.CFRule{{Type: "formula", Ranges: []model.Range{{R0: 0, C0: 1, R1: 0, C1: 1}}, Value1: "TRUE", Format: model.CFFormat{Fill: "FF0000"}}}
	got := svc.CellStyle(bg, 0, 0, 1)
	if got == nil || got.Fill != "FFFFFF" {
		t.Fatalf("raw format = %+v", got)
	}
	if len(rec.events) != 0 || sess.doc.Dirty() {
		t.Fatal("reading cell format modified document")
	}
	if svc.CellStyle(bg, 99, 0, 0) != nil || svc.CellStyle(bg, 0, -1, 0) != nil {
		t.Fatal("invalid cell style query")
	}
}

func TestFillOverlayCacheSeparatesTableAndConditionalColor(t *testing.T) {
	sess, _ := newTestSession(false)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	st := wb.Styles.Get(0)
	st.Fill = "FFFFFF"
	id := wb.Styles.Intern(st)
	sh.Layout.Tables = []model.Table{{Name: "Sales", Style: "TableStyleMedium2", Range: model.Range{R0: 0, C0: 0, R1: 3, C1: 2}}}
	sh.Layout.CondFormats = []model.CFRule{{Type: "formula", Ranges: []model.Range{{R0: 1, C0: 1, R1: 1, C1: 1}}, Value1: "TRUE", Format: model.CFFormat{Fill: "D9E1F2"}}}
	look := newLook(sess.doc, 0)
	for _, tc := range []struct {
		r, c int
		want string
	}{{1, 0, "FFFFFF"}, {1, 1, "D9E1F2"}, {1, 2, "FFFFFF"}} {
		actual, _, _, _, _ := look.style(tc.r, tc.c, model.StringCell("x"), id)
		if wb.Styles.Get(actual).Fill != tc.want {
			t.Fatalf("%d,%d fill cache returned incorrect priority", tc.r, tc.c)
		}
	}
}
