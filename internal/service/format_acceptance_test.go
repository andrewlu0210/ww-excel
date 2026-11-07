package service

import (
	"bytes"
	"path/filepath"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
)

func TestFormatAcceptancePartialMergeUsesLogicalCell(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	sh.Layout.Merges = []model.Range{{R0: 0, C0: 0, R1: 1, C1: 2}}
	sh.Set(0, 0, model.StringCell("merged"))
	fill := "FFFF00"
	size := 18.0
	wrap := true
	if err := svc.Format(bg, 0, []RangeDTO{{1, 1, 1, 1}}, FormatPatch{Fill: &fill, FontSize: &size, Wrap: &wrap}); err != nil {
		t.Fatal(err)
	}
	for r := 0; r <= 1; r++ {
		for c := 0; c <= 2; c++ {
			st := wb.Styles.Get(sh.EffectiveStyle(r, c))
			if st.Fill != fill || st.Font.Size != size || !st.Align.Wrap {
				t.Fatalf("partial merge (%d,%d) = %+v", r, c, st)
			}
		}
	}
	svc.Undo(bg)
	if sh.Get(0, 0).Str != "merged" || wb.Styles.Get(sh.EffectiveStyle(0, 0)).Fill != "" || len(sh.Layout.Merges) != 1 {
		t.Fatal("merged format undo")
	}
	svc.Redo(bg)
	if wb.Styles.Get(sh.EffectiveStyle(0, 0)).Fill != fill {
		t.Fatal("merged format redo")
	}
}

func TestFormatAcceptanceSelectionMatrix(t *testing.T) {
	cases := []struct {
		name   string
		ranges []RangeDTO
		inside [][2]int
		merge  bool
	}{
		{"single", []RangeDTO{{3, 3, 3, 3}}, [][2]int{{3, 3}}, false},
		{"multiple", []RangeDTO{{3, 3, 5, 5}}, [][2]int{{3, 3}, {4, 4}, {5, 5}}, false},
		{"disjoint", []RangeDTO{{3, 3, 3, 4}, {6, 6, 7, 7}}, [][2]int{{3, 3}, {3, 4}, {7, 7}}, false},
		{"overlapping", []RangeDTO{{3, 3, 4, 4}, {4, 4, 5, 5}}, [][2]int{{3, 3}, {4, 4}, {5, 5}}, false},
		{"merged", []RangeDTO{{3, 3, 3, 3}}, [][2]int{{2, 2}, {3, 3}, {3, 4}}, true},
		{"whole columns", []RangeDTO{{0, 2, model.MaxRows - 1, 3}}, [][2]int{{0, 2}, {4, 3}, {900000, 2}, {model.MaxRows - 1, 3}}, false},
		{"whole rows", []RangeDTO{{2, 0, 3, model.MaxCols - 1}}, [][2]int{{2, 0}, {3, 3}, {2, 9000}, {3, model.MaxCols - 1}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess, _ := newTestSession(false)
			svc := NewSheetService(sess)
			wb := sess.doc.Workbook()
			sh := wb.Sheets[0]
			rowStyle := wb.Styles.Get(0)
			rowStyle.Font.Name = "Arial"
			rowStyle.Fill = "EEEEDD"
			sh.Layout.RowStyles[3] = wb.Styles.Intern(rowStyle)
			colStyle := wb.Styles.Get(0)
			colStyle.Font.Italic = true
			sh.Layout.ColStyles[3] = wb.Styles.Intern(colStyle)
			sh.Set(3, 3, model.StringCell("original"))
			sh.Set(6, 6, model.NumberCell(123.5))
			if tc.merge {
				sh.Layout.Merges = []model.Range{{R0: 2, C0: 2, R1: 3, C1: 4}}
			}
			before := map[[2]int]model.Style{}
			for _, rc := range tc.inside {
				before[rc] = wb.Styles.Get(sh.EffectiveStyle(rc[0], rc[1]))
			}
			outside := wb.Styles.Get(sh.EffectiveStyle(9, 9))
			fill := "00FF00"
			size := 16.0
			align := "center"
			num := "#,##0.00"
			outer := &BorderLineDTO{Style: "double", Color: "FF0000"}
			inner := &BorderLineDTO{Style: "mediumDashDot", Color: "0070C0"}
			err := svc.Format(bg, 0, tc.ranges, FormatPatch{Fill: &fill, FontSize: &size, AlignH: &align, NumFmt: &num, Borders: &BorderPatchDTO{Top: outer, Bottom: outer, Left: outer, Right: outer, InsideH: inner, InsideV: inner}})
			if err != nil {
				t.Fatal(err)
			}
			validate := func(book *model.Workbook) {
				t.Helper()
				sheet := book.Sheets[0]
				for _, rc := range tc.inside {
					st := book.Styles.Get(sheet.EffectiveStyle(rc[0], rc[1]))
					if st.Fill != fill || st.Font.Size != size || st.Align.H != align || st.NumFmt != num {
						t.Fatalf("%v wrong format: %+v", rc, st)
					}
					if st.Font.Name != before[rc].Font.Name || st.Font.Italic != before[rc].Font.Italic {
						t.Fatalf("%v inherited font overwritten", rc)
					}
				}
				if book.Styles.Get(sheet.EffectiveStyle(9, 9)) != outside {
					t.Fatal("unselected gap changed")
				}
			}
			validate(wb)
			if sh.BlockCount() > 16 {
				t.Fatal("whole-axis operation allocated excessive blocks")
			}
			svc.Undo(bg)
			for _, rc := range tc.inside {
				if wb.Styles.Get(sh.EffectiveStyle(rc[0], rc[1])) != before[rc] {
					t.Fatalf("undo %v", rc)
				}
			}
			if sess.State().CanUndo {
				t.Fatal("format split into multiple undo entries")
			}
			svc.Redo(bg)
			validate(wb)
			var data bytes.Buffer
			if err = wwx.Write(&data, wb); err != nil {
				t.Fatal(err)
			}
			w, err := wwx.Read(&data)
			if err != nil {
				t.Fatal(err)
			}
			validate(w)
			path := filepath.Join(t.TempDir(), "matrix.xlsx")
			f, err := xlsxfile.Save(path, wb, nil, xlsxfile.Changes{})
			if err != nil {
				t.Fatal(err)
			}
			f.Close()
			x, src, err := xlsxfile.Read(path)
			if err != nil {
				t.Fatal(err)
			}
			src.Close()
			validate(x)
		})
	}
}
