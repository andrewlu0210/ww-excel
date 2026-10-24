package parser

import (
	"strings"
	"testing"

	"ww-excel/internal/formula/ast"
)

func TestParseCanonical(t *testing.T) {
	tests := []struct{ in, want string }{
		{"1+2*3", "1+2*3"},
		{"sum(a1:b2)", "SUM(A1:B2)"},
		{"SUM( A1 , $b$2 )", "SUM(A1,$B$2)"},
		{"B2:A1", "A1:B2"},
		{"$B$2:A1", "A1:$B$2"},
		{"a:c", "A:C"},
		{"$1:3", "$1:3"},
		{"Sheet2!A1:B3", "Sheet2!A1:B3"},
		{"'My Sheet'!A1", "'My Sheet'!A1"},
		{"工作表1!A1", "工作表1!A1"},
		{`"a""b"&"c"`, `"a""b"&"c"`},
		{"-A1%", "-A1%"},
		{"IF(A1>0,,\"neg\")", `IF(A1>0,,"neg")`},
		{"{1,-2;\"x\",TRUE}", `{1,-2;"x",TRUE}`},
		{"#n/a", "#N/A"},
		{"true", "TRUE"},
		{"LOG10(100)", "LOG10(100)"}, // looks like a cell ref but is a function
		{"STDEV.S(A1:A3)", "STDEV.S(A1:A3)"},
		{"myName+1", "myName+1"},
		{"SUM(A1:A3", "SUM(A1:A3)"}, // auto-close
		{"ROUND(SUM(A1,2", "ROUND(SUM(A1,2))"},
		{"1.5E+3", "1500"},
		{"A1 <> B1", "A1<>B1"},
		{"NOW()", "NOW()"},
	}
	for _, tt := range tests {
		n, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.in, err)
			continue
		}
		if got := ast.Format(n); got != tt.want {
			t.Errorf("Parse(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPrecedence(t *testing.T) {
	// Parenthesize the structure to check precedence and associativity.
	var show func(ast.Node) string
	show = func(n ast.Node) string {
		switch x := n.(type) {
		case ast.Binary:
			return "(" + show(x.L) + x.Op + show(x.R) + ")"
		case ast.Unary:
			return "(" + string(x.Op) + show(x.X) + ")"
		case ast.Percent:
			return "(" + show(x.X) + "%)"
		default:
			return ast.Format(n)
		}
	}
	tests := []struct{ in, want string }{
		{"-2^2", "((-2)^2)"},
		{"2^3^2", "((2^3)^2)"},
		{"1+2*3-4", "((1+(2*3))-4)"},
		{"1&2+3", "(1&(2+3))"},
		{"1+2=3", "((1+2)=3)"},
		{"50%*2", "((50%)*2)"},
		{"-A1:B2", "(-A1:B2)"},
		{"1-2-3", "((1-2)-3)"},
	}
	for _, tt := range tests {
		n, err := Parse(tt.in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.in, err)
		}
		if got := show(n); got != tt.want {
			t.Errorf("Parse(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct{ in, wantMsg string }{
		{"", "空"},
		{"1+", "缺少運算元"},
		{"(1+2))", "多了一個"},
		{"1 2", "缺少運算子"},
		{`"abc`, "引號"},
		{"SUM(1;2)", "「)」或「,」"},
		{"A1:SUM(B1)", "儲存格參照"},
		{"{1,2;3}", "長度"},
		{"{A1}", "陣列常數"},
		{"#FOO", "錯誤值"},
		{"Sheet1!5", "儲存格參照"},
		{"A$", "無效的參照"},
		{"1@", "無法辨識"},
	}
	for _, tt := range tests {
		_, err := Parse(tt.in)
		if err == nil {
			t.Errorf("Parse(%q) should fail", tt.in)
			continue
		}
		if !strings.Contains(err.Error(), tt.wantMsg) {
			t.Errorf("Parse(%q) error %q, want it to mention %q", tt.in, err, tt.wantMsg)
		}
	}
}

func TestFilePrefixesDropped(t *testing.T) {
	n, err := Parse("_xlfn.XLOOKUP(A1,B:B,C:C)+_xlfn._xlws.SORT(A1)")
	if err != nil {
		t.Fatal(err)
	}
	if got := ast.Format(n); got != "XLOOKUP(A1,B:B,C:C)+SORT(A1)" {
		t.Fatalf("got %q", got)
	}
}
