package printing

import (
	"bytes"
	"compress/zlib"
	"github.com/signintech/gopdf"
	"io"
	"regexp"
	"strings"
	"testing"
	"ww-excel/internal/model"
)

func borderPDFContent(t *testing.T, data []byte) string {
	t.Helper()
	var result strings.Builder
	for _, m := range regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`).FindAllSubmatch(data, -1) {
		z, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue
		}
		decoded, err := io.ReadAll(io.LimitReader(z, 1<<20))
		z.Close()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(decoded, []byte(" RG")) {
			result.Write(decoded)
		}
	}
	return result.String()
}
func TestPDFBorderVariantsRetainDashAndWeight(t *testing.T) {
	for _, tc := range []struct{ style, weight, dash string }{
		{"mediumDashed", "1.25 w", "[6.00 2.00] 0.00 d"},
		{"dashDot", "0.50 w", "[6.00 2.00 2.00 2.00] 0.00 d"},
		{"mediumDashDotDot", "1.25 w", "[6.00 2.00 2.00 2.00 2.00 2.00] 0.00 d"},
	} {
		t.Run(tc.style, func(t *testing.T) {
			pdf := &gopdf.GoPdf{}
			pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
			pdf.AddPage()
			r := renderer{pdf: pdf}
			r.borders(model.Style{Border: model.Borders{Bottom: model.BorderLine{Style: tc.style, Color: "123456"}}}, 10, 10, 100, 30)
			data, err := pdf.GetBytesPdfReturnErr()
			if err != nil {
				t.Fatal(err)
			}
			content := borderPDFContent(t, data)
			if !strings.Contains(content, tc.weight) || !strings.Contains(content, tc.dash) {
				t.Fatalf("PDF lost border variant: %q", content)
			}
		})
	}
}

func TestPDFSlantedBorderUsesDiagonalSegments(t *testing.T) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()
	r := renderer{pdf: pdf}
	r.borders(model.Style{Border: model.Borders{Bottom: model.BorderLine{Style: "slantDashDot", Color: "123456"}}}, 10, 10, 19, 30)
	data, err := pdf.GetBytesPdfReturnErr()
	if err != nil {
		t.Fatal(err)
	}
	content := borderPDFContent(t, data)
	if strings.Count(content, " l S\n") < 2 || strings.Contains(content, "[6.00 2.00 2.00 2.00]") {
		t.Fatalf("slanted strokes missing: %q", content)
	}
}

func TestPDFDiagonalDirections(t *testing.T) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()
	r := renderer{pdf: pdf}
	r.borders(model.Style{Border: model.Borders{Diagonal: model.BorderLine{Style: "thin", Color: "FF0000"}, DiagonalUp: true, DiagonalDown: true}}, 10, 20, 100, 80)
	data, err := pdf.GetBytesPdfReturnErr()
	if err != nil {
		t.Fatal(err)
	}
	content := borderPDFContent(t, data)
	matches := regexp.MustCompile(`([0-9.]+) ([0-9.]+) m ([0-9.]+) ([0-9.]+) l S`).FindAllStringSubmatch(content, -1)
	if len(matches) != 2 {
		t.Fatalf("expected two diagonal strokes: %s", content)
	}
	for _, m := range matches {
		if m[1] == m[3] || m[2] == m[4] {
			t.Fatal("diagonal rendered as axis-aligned edge")
		}
	}
}
