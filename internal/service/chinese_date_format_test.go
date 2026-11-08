package service

import (
	"bytes"
	"path/filepath"
	"testing"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
)

func TestChineseDateFormatDisplayAndRoundTrips(t *testing.T) {
	cases := []struct{ code, want string }{
		{`m"月"d"日"`, "3月14日"},
		{`[DBNum1][$-404]yyyy"年"m"月"d"日"`, "二○一二年三月十四日"},
		{`[DBNum1][$-404]m"月"d"日"`, "三月十四日"},
		{`[$-404]dddd`, "星期三"},
		{`[$-404]ddd`, "週三"},
	}
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	wb := sess.doc.Workbook()
	sh := wb.Sheets[0]
	for r, tc := range cases {
		sh.Set(r, 0, model.NumberCell(40982))
		if got := svc.PreviewFormat(bg, 0, r, 0, tc.code); got != tc.want {
			t.Fatalf("preview %s: %s", tc.code, got)
		}
		if err := svc.Format(bg, 0, []RangeDTO{{r, 0, r, 0}}, FormatPatch{NumFmt: &tc.code}); err != nil {
			t.Fatal(err)
		}
	}
	verify := func(book *model.Workbook) {
		t.Helper()
		sheet := book.Sheets[0]
		for r, tc := range cases {
			c := sheet.Get(r, 0)
			st := book.Styles.Get(sheet.EffectiveStyle(r, 0))
			if c.Num != 40982 || st.NumFmt != tc.code {
				t.Fatalf("date value/code changed: %+v %s", c, st.NumFmt)
			}
			if text, _ := display(c, st); text != tc.want {
				t.Fatalf("display %s want %s", text, tc.want)
			}
		}
	}
	verify(wb)
	source := printSource{wb: wb, sh: sh, look: newLook(sess.doc, 0)}
	for r, tc := range cases {
		v, ok := source.View(r, 0)
		if !ok || v.Text != tc.want {
			t.Fatalf("print source %d: %+v", r, v)
		}
	}
	if pdf, _, err := renderPDF(sess, 0, nil); err != nil || len(pdf) == 0 {
		t.Fatal("Chinese date PDF", err)
	}
	var data bytes.Buffer
	if err := wwx.Write(&data, wb); err != nil {
		t.Fatal(err)
	}
	w, err := wwx.Read(&data)
	if err != nil {
		t.Fatal(err)
	}
	verify(w)
	path := filepath.Join(t.TempDir(), "chinese-dates.xlsx")
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
	verify(x)
}
