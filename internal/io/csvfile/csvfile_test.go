package csvfile

import (
	"bytes"
	"strings"
	"testing"

	"ww-excel/internal/model"
)

func TestReadExcelDialect(t *testing.T) {
	in := "\xEF\xBB\xBFName,Qty,Note\r\n台北,12.5,\"a, b\"\r\n,,\"line1\nline2\"\r\n"
	res, err := Read(strings.NewReader(in), "S")
	if err != nil {
		t.Fatal(err)
	}
	s := res.Sheet
	checks := []struct {
		row, col int
		want     model.Cell
	}{
		{0, 0, model.StringCell("Name")}, // BOM stripped
		{1, 0, model.StringCell("台北")},
		{1, 1, model.NumberCell(12.5)},
		{1, 2, model.StringCell("a, b")},
		{2, 0, model.Cell{}},
		{2, 2, model.StringCell("line1\nline2")},
	}
	for _, c := range checks {
		if got := s.Get(c.row, c.col); got != c.want {
			t.Errorf("(%d,%d) = %+v, want %+v", c.row, c.col, got, c.want)
		}
	}
	if res.Truncated {
		t.Error("unexpected truncation")
	}
}

func TestRoundTrip(t *testing.T) {
	s := model.NewSheet("S")
	s.Set(0, 0, model.StringCell("x"))
	s.Set(0, 3, model.NumberCell(-1.25))
	s.Set(4, 1, model.StringCell(`quote " and, comma`))
	s.Set(4, 2, model.StringCell("多行\n文字"))

	var buf bytes.Buffer
	if err := Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), bom) {
		t.Fatal("missing BOM")
	}
	if !strings.Contains(buf.String(), "x,,,-1.25\r\n") {
		t.Fatalf("unexpected output:\n%q", buf.String())
	}

	res, err := Read(&buf, "S")
	if err != nil {
		t.Fatal(err)
	}
	for _, rc := range [][2]int{{0, 0}, {0, 3}, {4, 1}, {4, 2}, {2, 0}} {
		if got, want := res.Sheet.Get(rc[0], rc[1]), s.Get(rc[0], rc[1]); got != want {
			t.Errorf("(%d,%d) = %+v, want %+v", rc[0], rc[1], got, want)
		}
	}
}

func TestSingleColumnGapsSurvive(t *testing.T) {
	s := model.NewSheet("S")
	s.Set(0, 0, model.NumberCell(1))
	s.Set(3, 0, model.NumberCell(4))

	var buf bytes.Buffer
	if err := Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	res, err := Read(&buf, "S")
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Sheet.Get(3, 0); got != model.NumberCell(4) {
		t.Fatalf("row 4 = %+v; empty rows collapsed", got)
	}
}

func TestEmptySheet(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, model.NewSheet("S")); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), bom) {
		t.Fatalf("empty sheet wrote %q", buf.String())
	}
}
