// Package testutil builds small, self-contained workbook fixtures for tests.
package testutil

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// PivotXLSX creates a real pivot workbook, optionally corrupting one XML part.
func PivotXLSX(t testing.TB, mutation string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	for i, row := range [][]any{{"Product", "Amount"}, {"Beer", 1400}, {"Wine", 1600}} {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			t.Fatal(err)
		}
		if err = f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.AddPivotTable(&excelize.PivotTableOptions{
		Name: "SalesPivot", DataRange: "Sheet1!A1:B3", PivotTableRange: "Sheet1!D1:F8",
		Rows: []excelize.PivotTableField{{Data: "Product"}}, Data: []excelize.PivotTableField{{Data: "Amount", Subtotal: "Sum", Name: "Total"}},
	}); err != nil {
		t.Fatal(err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	src, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var result bytes.Buffer
	out := zip.NewWriter(&result)
	changed := mutation == "valid"
	for _, part := range src.File {
		in, err := part.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(in)
		in.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(part.Name, "xl/pivotTables/") && strings.HasSuffix(part.Name, ".xml") {
			switch mutation {
			case "negative-data-index", "out-of-range-data-index":
				index := "99999"
				if mutation == "negative-data-index" {
					index = "-1"
				}
				re := regexp.MustCompile(`(<dataField\b[^>]*\bfld=")\d+("[^>]*>)`)
				replaced := re.ReplaceAll(data, []byte("${1}"+index+"${2}"))
				changed = !bytes.Equal(data, replaced)
				data = replaced
			}
		}
		if strings.HasPrefix(part.Name, "xl/pivotCache/pivotCacheDefinition") && strings.HasSuffix(part.Name, ".xml") {
			switch mutation {
			case "empty-cache-fields", "short-cache-fields":
				fields := `<cacheFields count="0"></cacheFields>`
				if mutation == "short-cache-fields" {
					fields = `<cacheFields count="1"><cacheField name="Product"/></cacheFields>`
				}
				re := regexp.MustCompile(`(?s)<cacheFields\b[^>]*>.*?</cacheFields>`)
				replaced := re.ReplaceAll(data, []byte(fields))
				changed = !bytes.Equal(data, replaced)
				data = replaced
			}
		}
		dst, err := out.Create(part.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = dst.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err = out.Close(); err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatalf("fixture mutation %q did not change XML", mutation)
	}
	path := filepath.Join(t.TempDir(), "pivot.xlsx")
	if err = os.WriteFile(path, result.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
