// Generate an imported-shape workbook for native text editing verification.
package main

import (
	"flag"
	"github.com/xuri/excelize/v2"
	"log"
)

func main() {
	path := flag.String("o", "test_data/shape-edit.xlsx", "output workbook")
	flag.Parse()
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "文字方塊編輯驗收"
	must(f.SetSheetName("Sheet1", sheet))
	must(f.SetCellValue(sheet, "A1", "雙擊文字方塊，選取文字後使用常用工具列；⌘Enter 完成、Esc 取消"))
	must(f.AddShape(sheet, &excelize.Shape{Cell: "C4", Type: "roundRect", Width: 420, Height: 160, Fill: excelize.Fill{Color: []string{"E8F4EC"}}, Line: excelize.LineOptions{Fill: excelize.Fill{Color: []string{"39734D"}}, Width: 2}, Paragraph: []excelize.RichTextRun{{Text: "標題", Font: &excelize.Font{Family: "Arial", Size: 20, Color: "FF0000", Bold: true}}, {Text: "內文", Font: &excelize.Font{Family: "Arial", Size: 16, Color: "000000"}}}}))
	must(f.AddShape(sheet, &excelize.Shape{Cell: "K4", Type: "rect", Width: 300, Height: 160, Fill: excelize.Fill{Color: []string{"39734D"}}, Paragraph: []excelize.RichTextRun{{Text: "深入了解\n保留白色文字", Font: &excelize.Font{Size: 18, Color: "FFFFFF", Bold: true}}}}))
	_, err := f.NewSheet("其他頁")
	must(err)
	must(f.SaveAs(*path))
	log.Print(*path)
}
func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
