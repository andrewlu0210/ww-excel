package formula

import (
	"fmt"
	"strings"
	"testing"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/model"
)

func TestREPTRejectsOverflowAndRespectsTextLimits(t *testing.T) {
	for _, tc := range []struct{ name, text, count, want string }{
		{"overflow", strings.Repeat("a", 32768), "562949953421312", "#VALUE!"},
		{"ASCII boundary", "x", "32767", strings.Repeat("x", 32767)},
		{"ASCII too long", "x", "32768", "#VALUE!"},
		{"Unicode boundary", "中", "32767", strings.Repeat("中", 32767)},
		{"Unicode too long", "中", "32768", "#VALUE!"},
		{"empty huge count", "", "1000000000000000", ""},
		{"zero count overlong input", strings.Repeat("a", 32768), "0", ""},
		{"negative", "x", "-1", "#VALUE!"},
		{"fractional", "中", "2.9", "中中"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Errorf("REPT panicked: %v", p)
				}
			}()
			s := newSheet(t)
			s.set("A1", fmt.Sprintf("=REPT(%q,%s)", tc.text, tc.count))
			if got := s.get("A1"); got != tc.want {
				t.Fatalf("result lengths got=%d want=%d", len(got), len(tc.want))
			}
		})
	}
}

func TestParallelFormulaPanicsDoNotAbortOtherCells(t *testing.T) {
	wb := model.NewWorkbook()
	engine := New(wb)
	bad, err := engine.Compile("RAND()", 0)
	if err != nil {
		t.Fatal(err)
	}
	good, err := engine.Compile("1+2", 0)
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := engine.Compile("IFERROR(A1,42)", 0)
	if err != nil {
		t.Fatal(err)
	}
	for row := 0; row < parallelWave; row++ {
		wb.Sheets[0].Set(row, 0, model.Cell{F: bad})
		wb.Sheets[0].Set(row, 1, model.Cell{F: good})
	}
	wb.Sheets[0].Set(0, 2, model.Cell{F: dependent})
	engine.Rand = func() float64 { panic("injected parallel formula failure") }
	engine.Rebuild()
	for row := 0; row < parallelWave; row++ {
		if got := wb.Sheets[0].Get(row, 0).Display(); got != model.ErrValue {
			t.Fatalf("row %d panic result=%q", row, got)
		}
		if got := wb.Sheets[0].Get(row, 1).Num; got != 3 {
			t.Fatalf("healthy row %d result=%v", row, got)
		}
	}
	if got := wb.Sheets[0].Get(0, 2).Num; got != 42 {
		t.Fatal("dependent error handling stopped")
	}
	for _, run := range []func(ast.Node, int, int, int) eval.Value{engine.EvalAt, engine.ReferenceAt} {
		if got := run(ast.Call{Name: "RAND"}, 0, 0, 0); got.Str != model.ErrValue {
			t.Fatalf("direct formula evaluation escaped recovery: %+v", got)
		}
	}
	engine.Rand = func() float64 { return .5 }
	engine.Update([]Key{{Row: 0, Col: 0}})
	if got := wb.Sheets[0].Get(0, 0).Num; got != .5 {
		t.Fatal("parallel recomputation stopped")
	}
	if got := wb.Sheets[0].Get(0, 2).Num; got != .5 {
		t.Fatal("dependent recomputation stopped")
	}
}
