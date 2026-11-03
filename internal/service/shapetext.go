package service

import (
	"context"
	"errors"
	"math"
	"reflect"
	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

// UpdateShapeText replaces only text. Group children keep their geometry locks.
func (s *SheetService) UpdateShapeText(ctx context.Context, sheet, id int, runs []TextRunDTO) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		cur := d.Workbook().Sheets[sheet].Layout.ObjectByID(id)
		if cur == nil || cur.Shape == nil {
			return nil, errors.New("文字方塊不存在")
		}
		text := &model.RichText{}
		for _, run := range runs {
			f := run.Font
			if math.IsNaN(f.Size) || math.IsInf(f.Size, 0) || f.Size < 0 || f.Size > 409 {
				return nil, errors.New("字型大小必須介於 1 到 409")
			}
			font := model.Font{Name: f.Name, Size: f.Size, Color: f.Color, Bold: f.Bold, Italic: f.Italic, Underline: f.Underline, Strike: f.Strike}
			if font.Size == 0 {
				font.Size = cur.Shape.Font.Size
			}
			if font.Name == "" {
				font.Name = cur.Shape.Font.Name
			}
			text.Runs = append(text.Runs, model.TextRun{Text: run.Text, Font: model.RichFontOf(font)})
		}
		if reflect.DeepEqual(compactShapeRuns(richTextDTO(cur.Shape.Text, cur.Shape.Font)), compactShapeRuns(richTextDTO(text, cur.Shape.Font))) {
			return nil, nil
		}
		next := *cur
		shape := *cur.Shape
		shape.Text = text
		next.Shape = &shape
		return d.UpdateObjectCommand(sheet, next, "編輯文字方塊")
	})
}

// Equivalent run boundaries must not dirty an unchanged text box or discard
// original hyperlink/paragraph markup merely because the editor compacted it.
func compactShapeRuns(runs []TextRunDTO) []TextRunDTO {
	var out []TextRunDTO
	for _, run := range runs {
		if run.Text == "" {
			continue
		}
		if len(out) > 0 && out[len(out)-1].Font == run.Font {
			out[len(out)-1].Text += run.Text
		} else {
			out = append(out, run)
		}
	}
	if len(out) == 0 && len(runs) > 0 {
		out = append(out, runs[len(runs)-1])
	}
	return out
}
