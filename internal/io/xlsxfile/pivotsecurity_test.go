package xlsxfile

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/model"
	"ww-excel/internal/testutil"
)

func TestReadRejectsInvalidPivotFields(t *testing.T) {
	for _, mutation := range []string{"negative-data-index", "out-of-range-data-index", "empty-cache-fields", "short-cache-fields"} {
		t.Run(mutation, func(t *testing.T) {
			path := testutil.PivotXLSX(t, mutation)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("malformed pivot escaped as panic: %v", p)
				}
			}()
			wb, src, err := Read(path)
			if src != nil {
				defer src.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "樞紐") {
				t.Fatalf("expected a pivot validation error, got workbook=%v source=%v error=%v", wb != nil, src != nil, err)
			}
			if wb != nil || src != nil {
				t.Fatal("failed import returned a partially loaded workbook")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("failed import modified source")
			}
		})
	}
}

func TestReadValidPivotFields(t *testing.T) {
	wb, src, err := Read(testutil.PivotXLSX(t, "valid"))
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	pivots := wb.Sheets[0].Layout.Pivots
	if len(pivots) != 1 || pivots[0].Name != "SalesPivot" || len(pivots[0].Rows) != 1 || pivots[0].Rows[0] != "Product" || len(pivots[0].Values) != 1 || pivots[0].Values[0].Field != "Amount" {
		t.Fatalf("pivot fields changed: %+v", pivots)
	}
}

func TestReadContainsUnexpectedParserPanics(t *testing.T) {
	path := testutil.PivotXLSX(t, "valid")
	called := false
	wb, src, err := readXLSX(path, func(f *excelize.File, path string) (*model.Workbook, error) {
		called = true
		if f == nil {
			t.Fatal("source was not opened")
		}
		panic("parser failure")
	})
	if !called || err == nil || !strings.Contains(err.Error(), "parser failure") {
		t.Fatalf("panic did not become an import error: %v", err)
	}
	if wb != nil || src != nil {
		t.Fatal("recovered import returned partial data")
	}
	// A failed import must not prevent another file from being opened.
	wb, src, err = Read(path)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if len(wb.Sheets) != 1 {
		t.Fatal("subsequent import failed")
	}
}

func TestExcelizeDoesNotPanicOnInvalidPivotFields(t *testing.T) {
	for _, mutation := range []string{"negative-data-index", "out-of-range-data-index", "empty-cache-fields", "short-cache-fields"} {
		t.Run(mutation, func(t *testing.T) {
			src, err := excelize.OpenFile(testutil.PivotXLSX(t, mutation))
			if err != nil {
				t.Fatal(err)
			}
			defer src.Close()
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("Excelize pivot security patch regressed: %v", p)
				}
			}()
			// The dependency may either reject invalid fields or skip them safely.
			_, _ = src.GetPivotTables("Sheet1")
		})
	}
}
