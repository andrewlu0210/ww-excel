package wwx

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"ww-excel/internal/model"
)

func TestRoundTrip(t *testing.T) {
	wb := model.NewWorkbook()
	s := wb.Sheets[0]
	s.Set(0, 0, model.StringCell("42")) // text that looks like a number
	s.Set(0, 1, model.NumberCell(42))
	s.Set(model.MaxRows-1, model.MaxCols-1, model.StringCell("台北\n多行"))

	var buf bytes.Buffer
	if err := Write(&buf, wb); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	gs := got.Sheets[0]
	if gs.Name != "Sheet1" {
		t.Errorf("name = %q", gs.Name)
	}
	for _, rc := range [][2]int{{0, 0}, {0, 1}, {model.MaxRows - 1, model.MaxCols - 1}} {
		if a, b := gs.Get(rc[0], rc[1]), s.Get(rc[0], rc[1]); a != b {
			t.Errorf("(%d,%d) = %+v, want %+v", rc[0], rc[1], a, b)
		}
	}
}

func gz(s string) *bytes.Buffer {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write([]byte(s))
	zw.Close()
	return &buf
}

func TestReadRejects(t *testing.T) {
	tests := map[string]*bytes.Buffer{
		"not gzip":     bytes.NewBufferString("hello"),
		"wrong format": gz(`{"format":"other","version":1,"sheets":[{"name":"a","cells":[]}]}`),
		"newer":        gz(`{"format":"ww-excel","version":99,"sheets":[{"name":"a","cells":[]}]}`),
		"no sheets":    gz(`{"format":"ww-excel","version":1,"sheets":[]}`),
		"out of range": gz(`{"format":"ww-excel","version":1,"sheets":[{"name":"a","cells":[{"r":-1,"c":0,"n":1}]}]}`),
	}
	for name, in := range tests {
		if _, err := Read(in); err == nil {
			t.Errorf("%s: expected error", name)
		} else if !strings.Contains(err.Error(), "wwx") && !strings.Contains(err.Error(), "range") {
			t.Errorf("%s: unhelpful error %v", name, err)
		}
	}
}
