package numfmt

import "testing"

func TestLiteralNumericSections(t *testing.T) {
	for _, tc := range []struct {
		v    float64
		code string
		want bool
	}{
		{1, `"完成";"";""`, true}, {0, `"完成";"";""`, true},
		{1, `"NT$"#,##0`, false}, {1, `"0#?"`, true}, {1, `0%`, false},
		{46000, `yyyy/m/d`, false}, {1, `General`, false}, {1, ``, false},
	} {
		if got := IsLiteral(tc.v, tc.code); got != tc.want {
			t.Errorf("IsLiteral(%v,%q)=%v", tc.v, tc.code, got)
		}
	}
}
