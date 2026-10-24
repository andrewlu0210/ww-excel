// Command bigfile writes a large xlsx for load-time measurements:
// rows × 10 columns of numbers, text and formulas.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	rows := flag.Int("rows", 100000, "data rows")
	out := flag.String("o", "big.xlsx", "output file")
	flag.Parse()
	f := excelize.NewFile()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		log.Fatal(err)
	}
	header := []any{"ID", "名稱", "數量", "單價", "金額", "日期", "類別", "備註", "比率", "累計"}
	if err := sw.SetRow("A1", header); err != nil {
		log.Fatal(err)
	}
	for r := 2; r <= *rows+1; r++ {
		row := []any{
			r - 1, fmt.Sprintf("品項%d", r%997), r % 50, float64(r%300) + 0.5,
			excelize.Cell{Formula: fmt.Sprintf("C%d*D%d", r, r)}, 45000 + r%3000, []string{"甲", "乙", "丙"}[r%3],
			"", float64(r%100) / 100, excelize.Cell{Formula: fmt.Sprintf("E%d+J%d", r, r-1)},
		}
		if r == 2 {
			row[9] = excelize.Cell{Formula: "E2"}
		}
		cell, _ := excelize.CoordinatesToCellName(1, r)
		if err := sw.SetRow(cell, row); err != nil {
			log.Fatal(err)
		}
	}
	if err := sw.Flush(); err != nil {
		log.Fatal(err)
	}
	if err := f.SaveAs(*out); err != nil {
		log.Fatal(err)
	}
}
