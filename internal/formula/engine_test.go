package formula

import (
	"strings"
	"testing"
	"time"

	"ww-excel/internal/model"
)

type sheetT struct {
	t  *testing.T
	wb *model.Workbook
	e  *Engine
}

func newSheet(t *testing.T) *sheetT {
	wb := &model.Workbook{Sheets: []*model.Sheet{model.NewSheet("Sheet1"), model.NewSheet("Data")}}
	return &sheetT{t: t, wb: wb, e: New(wb)}
}

// set enters text like a user would and recalculates; returns the
// cells whose values were recalculated.
func (s *sheetT) set(addr, text string) []Key {
	s.t.Helper()
	sheet, row, col := s.addr(addr)
	cell := model.ParseInput(text)
	if cell.F != nil {
		f, err := s.e.Compile(cell.F.Text, sheet)
		if err != nil {
			s.t.Fatalf("compile %s: %v", text, err)
		}
		cell.F = f
	}
	s.wb.Sheets[sheet].Set(row, col, cell)
	return s.e.Update([]Key{{Sheet: sheet, Row: row, Col: col}})
}

func (s *sheetT) get(addr string) string {
	sheet, row, col := s.addr(addr)
	return s.wb.Sheets[sheet].Get(row, col).Display()
}

func (s *sheetT) addr(addr string) (int, int, int) {
	sheet := 0
	if rest, ok := strings.CutPrefix(addr, "Data!"); ok {
		sheet, addr = 1, rest
	}
	row, col := parseAddr(s.t, addr)
	return sheet, row, col
}

func (s *sheetT) expect(addr, want string) {
	s.t.Helper()
	if got := s.get(addr); got != want {
		s.t.Errorf("%s = %q, want %q", addr, got, want)
	}
}

func TestPropagation(t *testing.T) {
	s := newSheet(t)
	s.set("A1", "1")
	s.set("A2", "=A1*10")
	s.set("A3", "=SUM(A1:A2)")
	s.set("B1", "=Data!A1+A3")
	s.expect("A3", "11")

	changed := s.set("A1", "2")
	s.expect("A2", "20")
	s.expect("A3", "22")
	s.expect("B1", "22")
	if len(changed) != 3 {
		t.Errorf("changed = %v, want A2, A3, B1", changed)
	}

	s.set("Data!A1", "100")
	s.expect("B1", "122")

	// A formula that no longer references A1 stops updating.
	s.set("A2", "=5")
	s.set("A1", "3")
	s.expect("A2", "5")
	s.expect("A3", "8")

	// Clearing a precedent recalculates dependents.
	s.set("A1", "")
	s.expect("A3", "5")
}

func TestRecalcKeepsStyle(t *testing.T) {
	s := newSheet(t)
	s.set("A1", "1")
	s.set("B1", "=A1*2")
	sh := s.wb.Sheets[0]
	c := sh.Get(0, 1)
	c.S = 7
	sh.Set(0, 1, c)
	s.set("A1", "5")
	if got := sh.Get(0, 1); got.Num != 10 || got.S != 7 {
		t.Fatalf("B1 after recalc = %+v, want value 10 with style 7", got)
	}
}

func TestCircular(t *testing.T) {
	s := newSheet(t)
	s.set("A1", "=B1+1")
	s.set("B1", "=A1+1")
	s.set("C1", "=A1+5") // downstream of the cycle
	s.set("D1", "=D1+1") // self reference
	s.expect("A1", "0")
	s.expect("B1", "0")
	s.expect("C1", "5")
	s.expect("D1", "0")
	if got := len(s.e.Circular()); got != 3 {
		t.Errorf("circular = %v, want A1, B1, D1", s.e.Circular())
	}
	// An unrelated edit keeps the cycles reported.
	s.set("Z9", "1")
	if got := len(s.e.Circular()); got != 3 {
		t.Errorf("after unrelated edit circular = %v", s.e.Circular())
	}

	// Breaking the cycle recalculates normally.
	s.set("D1", "1")
	s.set("B1", "3")
	s.expect("A1", "4")
	s.expect("C1", "9")
	if len(s.e.Circular()) != 0 {
		t.Errorf("cycle should be gone: %v", s.e.Circular())
	}
}

func TestVolatile(t *testing.T) {
	s := newSheet(t)
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	s.e.Now = func() time.Time { return now }
	s.set("A1", "=NOW()")
	s.set("A2", "=A1+1")
	first := s.get("A2")
	now = now.Add(time.Hour)
	s.set("Z9", "x") // any edit recalculates volatile formulas
	if s.get("A2") == first {
		t.Error("volatile formula and its dependents should recalculate")
	}
	s.set("B1", "=TODAY()")
	s.expect("B1", "46318")
}

func TestRebuildOnLoad(t *testing.T) {
	wb := &model.Workbook{Sheets: []*model.Sheet{model.NewSheet("Sheet1")}}
	sh := wb.Sheets[0]
	sh.Set(0, 0, model.NumberCell(4))
	sh.Set(1, 0, model.FormulaCell("A1*A1"))           // uncompiled, as read from a file
	unknown := model.NumberCell(42)                    // cached value from the file
	unknown.F = &model.Formula{Text: "[1]Sheet1!A1+1"} // external workbook reference
	sh.Set(2, 0, unknown)
	e := New(wb)
	e.Rebuild()
	if got := sh.Get(1, 0).Display(); got != "16" {
		t.Fatalf("A2 = %q", got)
	}
	if got := sh.Get(2, 0); got.Display() != "42" || got.F.Text != "[1]Sheet1!A1+1" {
		t.Fatalf("unsupported formula should keep its cached value: %+v", got)
	}
	if e.Formulas() != 2 {
		t.Fatalf("formulas = %d", e.Formulas())
	}
}

func TestCompileErrors(t *testing.T) {
	e := New(model.NewWorkbook())
	for text, want := range map[string]string{
		"SUM()":         "引數太少",
		"IF(1,2,3,4)":   "引數太多",
		"1+":            "缺少運算元",
		"ROUND(1)":      "引數太少",
		"NOSUCHFUNC(1)": "",
		"sum(a1:a3":     "",
	} {
		f, err := e.Compile(text, 0)
		if want == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", text, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %v, want %q", text, err, want)
		}
		_ = f
	}
	f, _ := e.Compile("sum(a1:a3", 0)
	if f.Text != "SUM(A1:A3)" {
		t.Errorf("canonical text = %q", f.Text)
	}
}

// chain builds A1=1, A2=A1+1, ..., An=A(n-1)+1.
func chain(t testing.TB, n int) (*model.Workbook, *Engine) {
	wb := model.NewWorkbook()
	e := New(wb)
	sh := wb.Sheets[0]
	sh.Set(0, 0, model.NumberCell(1))
	for r := 1; r < n; r++ {
		f, err := e.Compile("A"+itoa(r)+"+1", 0)
		if err != nil {
			t.Fatal(err)
		}
		sh.Set(r, 0, model.Cell{F: f})
	}
	e.Rebuild()
	return wb, e
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func TestChainRecalcUnder100ms(t *testing.T) {
	const n = 10_000
	wb, e := chain(t, n)
	sh := wb.Sheets[0]
	if got := sh.Get(n-1, 0).Display(); got != itoa(n) {
		t.Fatalf("A%d = %q", n, got)
	}
	start := time.Now()
	sh.Set(0, 0, model.NumberCell(100))
	changed := e.Update([]Key{{Row: 0, Col: 0}})
	elapsed := time.Since(start)
	if len(changed) != n-1 || sh.Get(n-1, 0).Display() != itoa(n+99) {
		t.Fatalf("changed %d cells, last = %q", len(changed), sh.Get(n-1, 0).Display())
	}
	t.Logf("10k chain recalc: %v", elapsed)
	if elapsed > 100*time.Millisecond {
		t.Errorf("10k chain recalc took %v, want < 100ms", elapsed)
	}
}

func TestDeepChainNoStackOverflow(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	wb, e := chain(t, 200_000)
	wb.Sheets[0].Set(0, 0, model.NumberCell(0))
	e.Update([]Key{{Row: 0, Col: 0}})
	if got := wb.Sheets[0].Get(199_999, 0).Display(); got != "199999" {
		t.Fatalf("last = %q", got)
	}
}

func BenchmarkChainRecalc10k(b *testing.B) {
	wb, e := chain(b, 10_000)
	sh := wb.Sheets[0]
	v := 0.0
	for b.Loop() {
		v++
		sh.Set(0, 0, model.NumberCell(v))
		e.Update([]Key{{Row: 0, Col: 0}})
	}
}

// 10k formulas each summing a 10-cell window: exercises range buckets.
func BenchmarkRangeSums10k(b *testing.B) {
	wb := model.NewWorkbook()
	e := New(wb)
	sh := wb.Sheets[0]
	for r := 0; r < 10_000; r++ {
		sh.Set(r, 0, model.NumberCell(float64(r)))
		f, _ := e.Compile("SUM(A"+itoa(r+1)+":A"+itoa(r+10)+")", 0)
		sh.Set(r, 1, model.Cell{F: f})
	}
	e.Rebuild()
	v := 0.0
	for b.Loop() {
		v++
		sh.Set(5000, 0, model.NumberCell(v))
		e.Update([]Key{{Row: 5000, Col: 0}})
	}
}
