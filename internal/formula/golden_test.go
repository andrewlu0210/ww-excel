package formula

import (
	"path/filepath"
	"strings"
	"testing"

	"ww-excel/internal/formula/golden"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

const goldenDir = "../../testdata/golden"

func parseAddr(t testing.TB, addr string) (int, int) {
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
	cases, err := golden.Load(filepath.Join(goldenDir, "functions.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	wb := golden.Fixture()
	e := New(wb)
	e.Rebuild()
	sheet := wb.Sheets[0]
	for _, c := range cases {
		compiled, err := e.Compile(strings.TrimPrefix(c.Formula, "="), 0)
		if err != nil {
			t.Errorf("line %d: %s: compile error: %v", c.Line, c.Formula, err)
			continue
		}
		sheet.Set(golden.CaseRow, golden.CaseCol, model.Cell{F: compiled})
		e.Update([]Key{{Sheet: 0, Row: golden.CaseRow, Col: golden.CaseCol}})
		if got := sheet.Get(golden.CaseRow, golden.CaseCol).Display(); got != c.Expected {
			t.Errorf("line %d: %s = %q, want %q", c.Line, c.Formula, got, c.Expected)
		}
	}
	if len(cases) < 200 {
		t.Fatalf("only %d golden cases", len(cases))
	}
	t.Logf("%d golden cases", len(cases))
}

// TestExcelGolden compares against values calculated by real Excel. The
// file comes from tools/goldenxlsx: open excel-check.xlsx in Excel and
// save it as excel-saved.xlsx (see testdata/golden/README.md).
func TestExcelGolden(t *testing.T) {
	path := filepath.Join(goldenDir, "excel-saved.xlsx")
	wb, src, err := xlsxfile.Read(path)
	if err != nil {
		t.Skipf("no Excel-saved golden file (%v); see testdata/golden/README.md", err)
	}
	defer src.Close()
	cases, err := golden.Load(filepath.Join(goldenDir, "functions.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	sheet := wb.Sheets[0]
	checked := 0
	for i, c := range cases {
		if c.PositionDependent() {
			continue
		}
		cell := sheet.Get(golden.CaseRow+i, golden.CaseCol)
		if cell.F == nil {
			t.Errorf("line %d: %s missing from the Excel file", c.Line, c.Formula)
			continue
		}
		excel := cell.Display()
		if cell.Kind == model.KindNumber {
			excel = numfmt.General(cell.Num)
		}
		if excel != c.Expected {
			t.Errorf("line %d: %s: Excel says %q, functions.tsv says %q", c.Line, c.Formula, excel, c.Expected)
		}
		checked++
	}
	t.Logf("checked %d cases against Excel", checked)
}
