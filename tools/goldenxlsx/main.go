// Command goldenxlsx writes testdata/golden/excel-check.xlsx: the golden
// fixture plus every case formula, one per row from Sheet1!Z1000, with
// the formula text in column AA and our expected result in column AB.
// Open it in Excel, then save it as excel-saved.xlsx; TestExcelGolden
// then compares Excel's results with functions.tsv.
package main

import (
	"fmt"
	"log"
	"strings"

	"ww-excel/internal/formula"
	"ww-excel/internal/formula/golden"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
)

func main() {
	cases, err := golden.Load("testdata/golden/functions.tsv")
	if err != nil {
		log.Fatal(err)
	}
	wb := golden.Fixture()
	e := formula.New(wb)
	sh := wb.Sheets[0]
	for i, c := range cases {
		row := golden.CaseRow + i
		f, err := e.Compile(strings.TrimPrefix(c.Formula, "="), 0)
		if err != nil {
			log.Fatalf("line %d: %v", c.Line, err)
		}
		sh.Set(row, golden.CaseCol, model.Cell{F: f})
		sh.Set(row, golden.CaseCol+1, model.StringCell(c.Formula))
		sh.Set(row, golden.CaseCol+2, model.StringCell(c.Expected))
	}
	e.Rebuild()
	const out = "testdata/golden/excel-check.xlsx"
	if _, err := xlsxfile.Save(out, wb, nil, xlsxfile.Changes{}); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s with %d cases\n", out, len(cases))
}
