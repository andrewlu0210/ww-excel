package document

import (
	"testing"
	"ww-excel/internal/model"
)

func TestHiddenSheetReferencesAndLastVisibleDeletion(t *testing.T) {
	wb := model.NewWorkbook()
	hidden := model.NewSheet("Data")
	hidden.Visibility = "hidden"
	hidden.Set(0, 0, model.NumberCell(42))
	wb.Sheets = append(wb.Sheets, hidden)
	wb.Sheets[0].Set(0, 0, model.Cell{F: &model.Formula{Text: "Data!A1"}})
	d := FromWorkbook(wb, "")
	defer d.Close()
	if got := wb.Sheets[0].Get(0, 0).Num; got != 42 {
		t.Fatal(got)
	}
	if _, err := d.DeleteSheetCommand(0); err == nil {
		t.Fatal("allowed deleting the last visible worksheet")
	}
	if _, err := d.DeleteSheetCommand(1); err != nil {
		t.Fatal(err)
	}
}
