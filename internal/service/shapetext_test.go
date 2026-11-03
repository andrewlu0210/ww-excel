package service

import (
	"reflect"
	"testing"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

func TestShapeTextEditKeepsGeometryAndUndo(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		wb := model.NewWorkbook()
		shape := &model.Shape{Geometry: "roundRect", Fill: "39734D", LineColor: "123456", LineWidth: 2, Left: 8, Top: 4, Rot: 15, Font: model.DefaultFont, Text: &model.RichText{Runs: []model.TextRun{{Text: "原文"}}}}
		original := model.Object{ID: 3, Kind: model.ObjectShape, Anchor: model.Anchor{Row: 2, Col: 3, DX: 7, DY: 9}, W: 240, H: 80, Shape: shape, ReadOnly: grouped}
		wb.Sheets[0].Layout.Objects = []model.Object{original}
		d := document.FromWorkbook(wb, "")
		svc := NewSheetService(NewSession(d, func(string, any) {}))
		font := fontDTO(model.DefaultFont)
		font.Bold = true
		font.Color = "FF0000"
		runs := []TextRunDTO{{Text: "標題🙂\n", Font: font}, {Text: "=123", Font: fontDTO(model.DefaultFont)}}
		if err := svc.UpdateShapeText(bg, 0, 3, runs); err != nil {
			t.Fatal(err)
		}
		got := *wb.Sheets[0].Layout.ObjectByID(3)
		if !reflect.DeepEqual(richTextDTO(got.Shape.Text, got.Shape.Font), runs) {
			t.Fatal("lost text formatting")
		}
		copy := got
		copy.Shape = shape
		if !reflect.DeepEqual(copy, original) {
			t.Fatal("changed object geometry")
		}
		if shape.Text.Runs[0].Text != "原文" {
			t.Fatal("mutated undo snapshot")
		}
		version := d.Version()
		if err := svc.UpdateShapeText(bg, 0, 3, runs); err != nil || d.Version() != version {
			t.Fatal("unchanged shape edit recorded")
		}
		svc.Undo(bg)
		if wb.Sheets[0].Layout.ObjectByID(3).Shape != shape {
			t.Fatal("undo lost original shape")
		}
		svc.Redo(bg)
		if wb.Sheets[0].Layout.ObjectByID(3).Shape.Text.Runs[0].Text != "標題🙂\n" {
			t.Fatal("redo lost text")
		}
		if err := svc.UpdateShapeText(bg, 0, 3, []TextRunDTO{}); err != nil || len(wb.Sheets[0].Layout.ObjectByID(3).Shape.Text.Runs) != 0 {
			t.Fatal("empty shape text not accepted")
		}
		if err := svc.UpdateShapeText(bg, 0, 999, runs); err == nil {
			t.Fatal("missing shape accepted")
		}
		d.Close()
	}
}

func TestShapeTextEquivalentRunBoundariesDoNotDirty(t *testing.T) {
	d := document.New()
	defer d.Close()
	font := model.DefaultFont
	d.Workbook().Sheets[0].Layout.Objects = []model.Object{{ID: 1, Kind: model.ObjectShape, Shape: &model.Shape{Font: font, Text: &model.RichText{Runs: []model.TextRun{{Text: "相同", Font: model.RichFontOf(font)}, {Text: "格式", Font: model.RichFontOf(font)}}}}}}
	svc := NewSheetService(NewSession(d, func(string, any) {}))
	version := d.Version()
	if err := svc.UpdateShapeText(bg, 0, 1, []TextRunDTO{{Text: "相同格式", Font: fontDTO(font)}}); err != nil || d.Version() != version {
		t.Fatal("equivalent run boundaries created an edit")
	}
}
