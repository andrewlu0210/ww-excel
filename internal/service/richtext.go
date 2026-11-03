package service

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"ww-excel/internal/model"
)

type TextRunDTO struct {
	Text string  `json:"text"`
	Font FontDTO `json:"font"`
}

func fontDTO(f model.Font) FontDTO {
	return FontDTO{Name: f.Name, Size: f.Size, Color: f.Color, Bold: f.Bold, Italic: f.Italic, Underline: f.Underline, Strike: f.Strike}
}

func richTextDTO(rich *model.RichText, base model.Font) []TextRunDTO {
	if rich == nil {
		return nil
	}
	out := make([]TextRunDTO, 0, len(rich.Runs))
	for _, run := range rich.Runs {
		out = append(out, TextRunDTO{Text: run.Text, Font: fontDTO(run.Font.Resolve(base))})
	}
	return out
}

// The edit string may include an apostrophe used to keep a literal value.
// Run offsets refer to that same string; the prefix is not stored in the cell.
func richInput(cell model.Cell, text string, runs []TextRunDTO, old model.Cell, base model.Font) (model.Cell, error) {
	var joined strings.Builder
	for _, run := range runs {
		joined.WriteString(run.Text)
		if math.IsNaN(run.Font.Size) || math.IsInf(run.Font.Size, 0) || run.Font.Size < 0 || run.Font.Size > 409 {
			return cell, fmt.Errorf("字型大小必須介於 1 到 409")
		}
	}
	if joined.String() != text || cell.F != nil {
		return cell, fmt.Errorf("分段格式僅適用於文字，且須與輸入內容一致")
	}
	// Text edited into a number, date or blank follows the ordinary input path.
	if cell.Kind != model.KindString {
		return cell, nil
	}
	skip := 0
	if strings.HasPrefix(text, "'") {
		skip = 1
	}
	rich := &model.RichText{}
	for _, run := range runs {
		value := run.Text
		if skip > 0 && value != "" {
			value = value[1:]
			skip = 0
		}
		if value == "" {
			continue
		}
		f := run.Font
		font := model.Font{Name: f.Name, Size: f.Size, Color: f.Color, Bold: f.Bold, Italic: f.Italic, Underline: f.Underline, Strike: f.Strike}
		rich.Runs = append(rich.Runs, model.TextRun{Text: value, Font: model.RichFontOf(font)})
	}
	cell.Rich = rich
	if old.Kind == model.KindString && old.Str == cell.Str && reflect.DeepEqual(richTextDTO(old.Rich, base), richTextDTO(rich, base)) {
		cell.Rich = old.Rich
	}
	return cell, nil
}
