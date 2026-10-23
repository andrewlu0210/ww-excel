package formula

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// fixture builds the workbook the golden formulas refer to.
func fixture(t *testing.T) (*model.Workbook, *Engine) {
	t.Helper()
	wb := &model.Workbook{Sheets: []*model.Sheet{model.NewSheet("Sheet1"), model.NewSheet("Sheet2")}}
	s := wb.Sheets[0]
	set := func(addr string, c model.Cell) {
		row, col := parseAddr(t, addr)
		s.Set(row, col, c)
	}
	for i, v := range []float64{10, 20, 30, 40, 50} {
		s.Set(i, 0, model.NumberCell(v)) // A1:A5
	}
	for i, v := range []string{"台北", "新竹", "台中", "台南", "高雄"} {
		s.Set(i, 1, model.StringCell(v)) // B1:B5
	}
	set("C1", model.NumberCell(1.5))
	set("C2", model.NumberCell(-2))
	set("C3", model.BoolCell(true))
	set("C4", model.StringCell("abc"))
	// C5 stays empty
	set("D1", model.NumberCell(numfmt.DateSerial(2026, 10, 23)))
	set("D2", model.NumberCell(0.75))
	set("D3", model.NumberCell(numfmt.DateSerial(2026, 10, 23)+0.5))
	for i, v := range []string{"apple", "banana", "cherry", "台北"} {
		s.Set(i, 4, model.StringCell(v)) // E1:E4
		s.Set(i, 5, model.NumberCell(float64((i+1)*100)))
	}
	for i, v := range []float64{1, 3, 5, 7, 9} {
		s.Set(i, 6, model.NumberCell(v)) // G1:G5
	}
	for i, v := range []string{"一", "三", "五", "七", "九"} {
		s.Set(i, 7, model.StringCell(v)) // H1:H5
	}
	set("I1", model.ErrorCell(model.ErrDiv0))
	set("J1", model.StringCell("  hello   world  "))
	wb.Sheets[1].Set(0, 0, model.NumberCell(7))

	e := New(wb)
	e.Rebuild()
	return wb, e
}

func parseAddr(t *testing.T, addr string) (int, int) {
	t.Helper()
	col := 0
	i := 0
	for ; i < len(addr) && addr[i] >= 'A' && addr[i] <= 'Z'; i++ {
		col = col*26 + int(addr[i]-'A'+1)
	}
	row := 0
	for ; i < len(addr); i++ {
		row = row*10 + int(addr[i]-'0')
	}
	return row - 1, col - 1
}

func TestGolden(t *testing.T) {
	f, err := os.Open("../../testdata/golden/functions.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	wb, e := fixture(t)
	const row, col = 999, 25 // Z1000
	sheet := wb.Sheets[0]

	sc := bufio.NewScanner(f)
	line, cases := 0, 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		formula, want, ok := strings.Cut(text, "\t")
		if !ok {
			t.Fatalf("line %d: missing tab", line)
		}
		if want == "(empty)" {
			want = ""
		}
		cases++
		compiled, err := e.Compile(strings.TrimPrefix(formula, "="), 0)
		if err != nil {
			t.Errorf("line %d: %s: compile error: %v", line, formula, err)
			continue
		}
		sheet.Set(row, col, model.Cell{F: compiled})
		e.Update([]Key{{Sheet: 0, Row: row, Col: col}})
		if got := sheet.Get(row, col).Display(); got != want {
			t.Errorf("line %d: %s = %q, want %q", line, formula, got, want)
		}
	}
	if cases < 200 {
		t.Fatalf("only %d golden cases", cases)
	}
	t.Logf("%d golden cases", cases)
}
