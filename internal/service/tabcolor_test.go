package service

import (
	"reflect"
	"testing"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func TestTabColorsFollowSheetsThroughStructureAndUndo(t *testing.T) {
	wb := model.NewWorkbook()
	wb.Sheets[0].Name = "A"
	wb.Sheets[0].Layout.TabColor = "123456"
	b := model.NewSheet("B")
	b.Layout.TabColor = "00AA00"
	b.Visibility = "hidden"
	wb.Sheets = append(wb.Sheets, b)
	sess := NewSession(document.FromWorkbook(wb, ""), func(string, any) {})
	defer sess.doc.Close()
	docs := NewDocumentService(sess, nil)
	sheets := NewSheetService(sess)
	if !reflect.DeepEqual(sess.State().TabColors, []string{"123456", "00AA00"}) {
		t.Fatal(sess.State())
	}
	if err := docs.RenameSheet(bg, 0, "Renamed"); err != nil {
		t.Fatal(err)
	}
	docs.MoveSheet(bg, 1, 0)
	state := sess.State()
	if !reflect.DeepEqual(state.TabColors, []string{"00AA00", "123456"}) || state.Sheets[1] != "Renamed" || len(state.HiddenSheets) != 1 || state.HiddenSheets[0] != 0 {
		t.Fatalf("colors detached from sheets: %+v", state)
	}
	sheets.Undo(bg)
	if !reflect.DeepEqual(sess.State().TabColors, []string{"123456", "00AA00"}) {
		t.Fatal("undo lost colors")
	}
	docs.AddSheet(bg, 2)
	if !reflect.DeepEqual(sess.State().TabColors, []string{"123456", "00AA00", ""}) {
		t.Fatal("new sheet inherited color")
	}
}
