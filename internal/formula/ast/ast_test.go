package ast

import (
	"testing"

	"ww-excel/internal/model"
)

func TestColNames(t *testing.T) {
	for i, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 701: "ZZ", 702: "AAA", 16383: "XFD"} {
		if got := ColName(i); got != want {
			t.Errorf("ColName(%d) = %q", i, got)
		}
		if got, ok := ColIndex(want); !ok || got != i {
			t.Errorf("ColIndex(%q) = %d, %v", want, got, ok)
		}
	}
	if _, ok := ColIndex("XFE"); ok {
		t.Error("XFE is past the last column")
	}
}

func TestFormat(t *testing.T) {
	n := Binary{Op: "+",
		L: Call{Name: "SUM", Args: []Node{Ref{Area: Span(CellAt(0, 0, false, false), CellAt(9, 1, true, true))}, Missing{}}},
		R: Paren{X: Unary{Op: '-', X: Percent{X: Number{V: 5}}}},
	}
	if got := Format(n); got != "SUM(A1:$B$10,)+(-5%)" {
		t.Errorf("Format = %q", got)
	}
	tests := map[string]Node{
		`"say ""hi"""`:     String{V: `say "hi"`},
		"'My Sheet'!$A:$C": Ref{Sheet: "My Sheet", Area: Area{Kind: ColArea, C0: 0, C1: 2, R1: model.MaxRows - 1, AbsC0: true, AbsC1: true}},
		"Sheet2!3:5":       Ref{Sheet: "Sheet2", Area: Area{Kind: RowArea, R0: 2, R1: 4, C1: model.MaxCols - 1}},
		"'A1'!B2":          Ref{Sheet: "A1", Area: CellAt(1, 1, false, false)},
		"工作表1!A1":          Ref{Sheet: "工作表1", Area: CellAt(0, 0, false, false)},
		"{1,2;\"a\",TRUE}": Array{Rows: [][]Node{{Number{V: 1}, Number{V: 2}}, {String{V: "a"}, Bool{V: true}}}},
		"1E+21":            Number{V: 1e21},
		"0.5":              Number{V: 0.5},
	}
	for want, n := range tests {
		if got := Format(n); got != want {
			t.Errorf("Format = %q, want %q", got, want)
		}
	}
}

func ref(r0, c0, r1, c1 int) Ref {
	return Ref{Area: Span(CellAt(r0, c0, false, false), CellAt(r1, c1, false, false))}
}

func TestAdjustRows(t *testing.T) {
	tests := []struct {
		name  string
		in    Node
		shift Shift
		want  string
	}{
		{"insert above moves", ref(4, 0, 4, 0), Shift{Sheet: "S", Rows: true, At: 2, Count: 3}, "A8"},
		{"insert below keeps", ref(4, 0, 4, 0), Shift{Sheet: "S", Rows: true, At: 10, Count: 3}, "A5"},
		{"insert inside grows range", ref(0, 0, 9, 0), Shift{Sheet: "S", Rows: true, At: 5, Count: 2}, "A1:A12"},
		{"delete before moves up", ref(9, 0, 9, 0), Shift{Sheet: "S", Rows: true, At: 2, Count: -3}, "A7"},
		{"delete cell gives #REF!", ref(3, 0, 3, 0), Shift{Sheet: "S", Rows: true, At: 3, Count: -1}, "#REF!"},
		{"delete inside shrinks", ref(0, 0, 9, 0), Shift{Sheet: "S", Rows: true, At: 2, Count: -3}, "A1:A7"},
		{"delete top of range", ref(2, 0, 9, 0), Shift{Sheet: "S", Rows: true, At: 0, Count: -5}, "A1:A5"},
		{"delete whole range", ref(2, 0, 4, 0), Shift{Sheet: "S", Rows: true, At: 1, Count: -10}, "#REF!"},
		{"other sheet untouched", Ref{Sheet: "T", Area: CellAt(4, 0, false, false)}, Shift{Sheet: "S", Rows: true, At: 0, Count: 1}, "T!A5"},
		{"columns ignore row shift", Ref{Area: Area{Kind: ColArea, C0: 1, C1: 1, R1: model.MaxRows - 1}}, Shift{Sheet: "S", Rows: true, At: 0, Count: -1}, "B:B"},
		{"insert columns", Binary{Op: "+", L: ref(0, 1, 0, 1), R: ref(0, 0, 0, 0)}, Shift{Sheet: "S", At: 1, Count: 2}, "D1+A1"},
		{"delete column under call", Call{Name: "SUM", Args: []Node{ref(0, 2, 0, 2)}}, Shift{Sheet: "s", At: 2, Count: -1}, "SUM(#REF!)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := Adjust(tt.in, "S", tt.shift)
			if s := Format(got); s != tt.want {
				t.Fatalf("Adjust = %q, want %q", s, tt.want)
			}
		})
	}
	if _, changed := Adjust(ref(0, 0, 0, 0), "S", Shift{Sheet: "S", Rows: true, At: 5, Count: 1}); changed {
		t.Fatal("untouched reference reported as changed")
	}
}

func TestTranslate(t *testing.T) {
	mixed := Ref{Area: Span(CellAt(0, 0, false, false), CellAt(4, 0, true, true))} // A1:$A$5
	tests := []struct {
		in     Node
		dr, dc int
		want   string
	}{
		{ref(0, 0, 0, 0), 2, 1, "B3"},
		{Ref{Area: CellAt(0, 0, true, true)}, 5, 5, "$A$1"},
		{Ref{Area: CellAt(0, 0, true, false)}, 5, 5, "F$1"},
		{mixed, 10, 0, "$A$5:A11"},
		{ref(0, 0, 0, 0), -1, 0, "#REF!"},
		{Ref{Area: Area{Kind: ColArea, C0: 0, C1: 1, R1: model.MaxRows - 1}}, 7, 1, "B:C"},
	}
	for _, tt := range tests {
		if got := Format(Translate(tt.in, tt.dr, tt.dc)); got != tt.want {
			t.Errorf("Translate(%s, %d, %d) = %q, want %q", Format(tt.in), tt.dr, tt.dc, got, tt.want)
		}
	}
}
