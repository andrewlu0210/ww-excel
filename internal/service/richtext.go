package service

import "ww-excel/internal/model"

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
