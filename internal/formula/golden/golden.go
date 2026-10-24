// Package golden holds the formula golden test data: the fixture
// workbook and the cases in testdata/golden/functions.tsv. It is shared
// by the engine tests and the tool that builds the Excel check file.
package golden

import (
	"bufio"
	"os"
	"strings"

	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// Cases are evaluated in Sheet1!Z1000 (row 999, column 25).
const CaseRow, CaseCol = 999, 25

type Case struct {
	Line     int
	Formula  string // with the leading "="
	Expected string // display text; "" for an empty string
}

// Fixture builds the workbook the golden formulas refer to.
func Fixture() *model.Workbook {
	wb := model.NewWorkbook()
	wb.Sheets = []*model.Sheet{model.NewSheet("Sheet1"), model.NewSheet("Sheet2")}
	s := wb.Sheets[0]
	for i, v := range []float64{10, 20, 30, 40, 50} {
		s.Set(i, 0, model.NumberCell(v)) // A1:A5
	}
	for i, v := range []string{"台北", "新竹", "台中", "台南", "高雄"} {
		s.Set(i, 1, model.StringCell(v)) // B1:B5
	}
	s.Set(0, 2, model.NumberCell(1.5)) // C1
	s.Set(1, 2, model.NumberCell(-2))
	s.Set(2, 2, model.BoolCell(true))
	s.Set(3, 2, model.StringCell("abc"))
	// C5 stays empty
	s.Set(0, 3, model.NumberCell(numfmt.DateSerial(2026, 10, 23))) // D1
	s.Set(1, 3, model.NumberCell(0.75))
	s.Set(2, 3, model.NumberCell(numfmt.DateSerial(2026, 10, 23)+0.5))
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
	s.Set(0, 8, model.ErrorCell(model.ErrDiv0))        // I1
	s.Set(0, 9, model.StringCell("  hello   world  ")) // J1
	wb.Sheets[1].Set(0, 0, model.NumberCell(7))        // Sheet2!A1
	return wb
}

// Load reads the cases from a functions.tsv file.
func Load(path string) ([]Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Case
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		formula, want, ok := strings.Cut(text, "\t")
		if !ok {
			continue
		}
		if want == "(empty)" {
			want = ""
		}
		out = append(out, Case{Line: line, Formula: formula, Expected: want})
	}
	return out, sc.Err()
}

// PositionDependent cases depend on the cell they are in (ROW(),
// COLUMN(), whole-range intersection), so they cannot be checked in a
// file that puts each case in its own row.
func (c Case) PositionDependent() bool {
	return strings.Contains(c.Formula, "ROW()") || strings.Contains(c.Formula, "COLUMN()") || c.Formula == "=A1:A5"
}
