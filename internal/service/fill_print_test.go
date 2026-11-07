package service

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strings"
	"testing"
	"ww-excel/internal/model"
)

// Read bounded, generated PDF content streams rather than comparing PDF bytes.
func fillPDFContent(t *testing.T, data []byte) string {
	t.Helper()
	var content strings.Builder
	for _, match := range regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`).FindAllSubmatch(data, -1) {
		reader, err := zlib.NewReader(bytes.NewReader(match[1]))
		if err != nil {
			continue
		}
		decoded, err := io.ReadAll(io.LimitReader(reader, 8<<20))
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(decoded, []byte(" re")) || bytes.Contains(decoded, []byte("BT\n")) {
			content.Write(decoded)
		}
	}
	if content.Len() == 0 {
		t.Fatal("PDF has no page content stream")
	}
	return content.String()
}
func TestFillPrintMatchesDisplayedStyle(t *testing.T) {
	for _, tc := range []struct {
		name, direct, conditional, want string
		table, merged                   bool
	}{
		{"solid", "112233", "", "0.067 0.133 0.200 rg", false, false},
		{"white", "FFFFFF", "", "1.000 1.000 1.000 rg", false, false},
		{"table direct white", "FFFFFF", "", "1.000 1.000 1.000 rg", true, false},
		{"table restored", "", "", "0.851 0.882 0.949 rg", true, false},
		{"conditional wins", "FFFFFF", "00FF00", "0.000 1.000 0.000 rg", true, false},
		{"merged empty", "112233", "", "0.067 0.133 0.200 rg", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, _ := newTestSession(false)
			svc := NewSheetService(sess)
			wb := sess.doc.Workbook()
			sh := wb.Sheets[0]
			if !tc.merged {
				sh.Set(1, 0, model.StringCell("fill"))
			} else {
				sh.Layout.Merges = []model.Range{{R0: 1, C0: 0, R1: 2, C1: 1}}
			}
			if tc.table {
				sh.Layout.Tables = []model.Table{{Name: "Sales", Style: "TableStyleMedium2", Range: model.Range{R0: 0, C0: 0, R1: 3, C1: 1}}}
			}
			if tc.conditional != "" {
				sh.Layout.CondFormats = []model.CFRule{{Type: "formula", Ranges: []model.Range{{R0: 1, C0: 0, R1: 1, C1: 0}}, Value1: "TRUE", Format: model.CFFormat{Fill: tc.conditional}}}
			}
			if err := svc.Format(bg, 0, []RangeDTO{{1, 0, 1, 0}}, FormatPatch{Fill: &tc.direct}); err != nil {
				t.Fatal(err)
			}
			source := printSource{wb: wb, sh: sh, look: newLook(sess.doc, 0)}
			view, ok := source.View(1, 0)
			if !ok || view.Style.Fill == "" {
				t.Fatal("print source omitted fill")
			}
			data, _, err := renderPDF(sess, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			content := fillPDFContent(t, data)
			if !strings.Contains(content, tc.want) {
				t.Fatalf("missing %s in PDF fill operators", tc.want)
			}
			if tc.conditional != "" && strings.Contains(content, "1.000 1.000 1.000 rg") {
				t.Fatal("direct white rendered over conditional color")
			}
		})
	}
}
func TestFillPrintNoFillOmitsPaint(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	sh := sess.doc.Workbook().Sheets[0]
	sh.Set(0, 0, model.StringCell("fill"))
	color := "112233"
	empty := ""
	svc.Format(bg, 0, []RangeDTO{{}}, FormatPatch{Fill: &color})
	svc.Format(bg, 0, []RangeDTO{{}}, FormatPatch{Fill: &empty})
	data, _, err := renderPDF(sess, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	content := fillPDFContent(t, data)
	if strings.Contains(content, "0.067 0.133 0.200 rg") || strings.Contains(content, "1.000 1.000 1.000 rg") {
		t.Fatal("no fill must not paint an opaque rectangle")
	}
}
