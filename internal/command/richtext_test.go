package command

import (
	"reflect"
	"testing"

	"ww-excel/internal/model"
)

func TestEditRichText(t *testing.T) {
	red, black, bold := "FF0000", "000000", true
	title := &model.RichFont{Color: &red, Bold: &bold}
	body := &model.RichFont{Color: &black}
	for _, tt := range []struct {
		name, text string
		want       []model.TextRun
	}{
		{"replace body", "標題說明", []model.TextRun{{Text: "標題", Font: title}, {Text: "說明", Font: body}}},
		{"insert at boundary", "標題：內文", []model.TextRun{{Text: "標題：", Font: title}, {Text: "內文", Font: body}}},
		{"prepend", "新標題內文", []model.TextRun{{Text: "新標題", Font: title}, {Text: "內文", Font: body}}},
		{"append emoji", "標題內文🙂", []model.TextRun{{Text: "標題", Font: title}, {Text: "內文🙂", Font: body}}},
		{"delete across runs", "標文", []model.TextRun{{Text: "標", Font: title}, {Text: "文", Font: body}}},
		{"replace across runs", "標新文", []model.TextRun{{Text: "標新", Font: title}, {Text: "文", Font: body}}},
		{"multiple edits", "新標題內文！", []model.TextRun{{Text: "新標題", Font: title}, {Text: "內文！", Font: body}}},
		{"complete replacement", "全新說明", []model.TextRun{{Text: "全新說明", Font: title}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wb := model.NewWorkbook()
			old := model.StringCell("標題內文")
			old.Rich = &model.RichText{Runs: []model.TextRun{{Text: "標題", Font: title}, {Text: "內文", Font: body}}}
			wb.Sheets[0].Set(0, 0, old)
			h := NewHistory(0)
			cmd := EnterText(wb, 0, 0, 0, tt.text)
			if cmd == nil {
				t.Fatal("edit unexpectedly ignored")
			}
			h.Exec(wb, cmd)
			got := wb.Sheets[0].Get(0, 0)
			if got.Str != tt.text || got.Rich == nil || !reflect.DeepEqual(got.Rich.Runs, tt.want) {
				t.Fatalf("edited cell = %+v, rich = %+v, want %+v", got, got.Rich, tt.want)
			}
			if !reflect.DeepEqual(old.Rich.Runs, []model.TextRun{{Text: "標題", Font: title}, {Text: "內文", Font: body}}) {
				t.Fatal("edit mutated the previous rich text")
			}
			if _, ok := h.Undo(wb); !ok || wb.Sheets[0].Get(0, 0) != old {
				t.Fatal("undo lost original text or formatting")
			}
			if _, ok := h.Redo(wb); !ok || wb.Sheets[0].Get(0, 0) != got {
				t.Fatal("redo lost edited text or formatting")
			}
		})
	}
}

func TestRichTextInputTypes(t *testing.T) {
	for _, text := range []string{"", "123", "TRUE", "=1+2", "'123", "unchanged"} {
		t.Run(text, func(t *testing.T) {
			wb := model.NewWorkbook()
			old := model.StringCell("unchanged")
			old.Rich = &model.RichText{Runs: []model.TextRun{{Text: old.Str}}}
			wb.Sheets[0].Set(0, 0, old)
			cmd := EnterText(wb, 0, 0, 0, text)
			if text == "unchanged" {
				if cmd != nil {
					t.Fatal("unchanged edit should not create history")
				}
				return
			}
			cmd.Do(wb)
			got := wb.Sheets[0].Get(0, 0)
			if text == "'123" {
				if got.Rich == nil || got.Str != "123" {
					t.Fatal("forced text should retain formatting")
				}
			} else if got.Rich != nil {
				t.Fatal("cleared, numeric, boolean and formula cells must not retain rich text")
			}
		})
	}
}
