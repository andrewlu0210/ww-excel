package model

import "testing"

func TestParseInput(t *testing.T) {
	tests := []struct {
		in   string
		want Cell
	}{
		{"", Cell{}},
		{"42", NumberCell(42)},
		{"-3.5", NumberCell(-3.5)},
		{" 1e3 ", NumberCell(1000)},
		{".5", NumberCell(0.5)},
		{"abc", StringCell("abc")},
		{"台北", StringCell("台北")},
		{"'42", StringCell("42")},
		{"'", Cell{}},
		{"Inf", StringCell("Inf")},
		{"NaN", StringCell("NaN")},
		{"0x1F", StringCell("0x1F")},
		{"1_000", StringCell("1_000")},
		{"=A1+1", StringCell("=A1+1")},
		{"line1\nline2", StringCell("line1\nline2")},
	}
	for _, tt := range tests {
		if got := ParseInput(tt.in); got != tt.want {
			t.Errorf("ParseInput(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestEditTextRoundTrip(t *testing.T) {
	for _, in := range []string{"42", "'42", "abc", "'abc", "-0.25", "  7"} {
		cell := ParseInput(in)
		if got := ParseInput(cell.EditText()); got != cell {
			t.Errorf("round trip %q: EditText=%q parsed=%+v want %+v", in, cell.EditText(), got, cell)
		}
	}
}
