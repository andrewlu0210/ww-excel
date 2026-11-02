// Generates a small Excel workbook for desktop rich-text editing regression.
package main

import (
	"flag"
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	path := flag.String("o", "test_data/richtext-edit.xlsx", "output workbook path")
	flag.Parse()
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "富文字編輯驗收"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		log.Fatal(err)
	}
	if err := f.SetCellRichText(sheet, "A1", []excelize.RichTextRun{
		{Text: "標題", Font: &excelize.Font{Family: "Arial", Size: 16, Color: "FF0000", Bold: true}},
		{Text: "內文", Font: &excelize.Font{Family: "Arial", Size: 16, Color: "000000"}},
	}); err != nil {
		log.Fatal(err)
	}
	if err := f.SetColWidth(sheet, "A", "A", 30); err != nil {
		log.Fatal(err)
	}
	if err := f.SetRowHeight(sheet, 1, 32); err != nil {
		log.Fatal(err)
	}
	if err := f.SaveAs(*path); err != nil {
		log.Fatal(err)
	}
	log.Print(*path)
}
