package service

import (
	"context"
	"sort"
	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

// FormatRowDTO supplies projected text metrics before an atomic format commit.
type FormatRowDTO struct {
	Row     int                `json:"row"`
	Minimum float64            `json:"minimum"`
	Cells   []FormatMeasureDTO `json:"cells"`
}
type FormatMeasureDTO struct {
	Text  string       `json:"text"`
	Style StyleDTO     `json:"style"`
	Runs  []TextRunDTO `json:"runs,omitempty"`
	Width float64      `json:"width"`
}

// FormatRows projects the edited fonts/wrapping without changing the document.
// Match the existing automatic-row scope: at most 500 rows per selected range.
func (s *SheetService) FormatRows(ctx context.Context, sheet int, ranges []RangeDTO, patch FormatPatch) ([]FormatRowDTO, error) {
	if len(ranges) > 4096 {
		return nil, command.ErrFillTooLarge
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []FormatRowDTO
	var err error
	s.at(ctx).read(func(d *document.Document) {
		sh := sheetAt(d, sheet)
		if sh == nil {
			return
		}
		wb := d.Workbook()
		selected := command.ExpandFormatRanges(toModelRanges(ranges), sh.Layout.Merges)
		rows := map[int]bool{}
		for _, rg := range selected {
			if rg.R1-rg.R0 > 500 {
				continue
			}
			for row := rg.R0; row <= rg.R1; row++ {
				if _, manual := sh.Layout.Rows[row]; !manual {
					rows[row] = true
				}
			}
		}
		if len(rows) > 512 {
			out = nil
			return
		}
		indices := make([]int, 0, len(rows))
		for row := range rows {
			indices = append(indices, row)
		}
		sort.Ints(indices)
		work := 0
		for _, row := range indices {
			if err = ctx.Err(); err != nil {
				return
			}
			result := FormatRowDTO{Row: row, Minimum: sh.Layout.DefaultRow, Cells: []FormatMeasureDTO{}}
			sh.EachInRange(model.Range{R0: row, C0: 0, R1: row, C1: model.MaxCols - 1}, func(r, col int, c model.Cell) {
				if err != nil || !c.HasContent() {
					return
				}
				work++
				if work > command.MaxFillCells {
					err = command.ErrFillTooLarge
					return
				}
				area := model.Range{R0: r, C0: col, R1: r, C1: col}
				if m, ok := sh.Layout.MergeAt(r, col); ok {
					if m.R1 > m.R0 || r != m.R0 || col != m.C0 {
						return
					}
					area = m
				}
				base := wb.Styles.Get(sh.EffectiveStyle(r, col))
				next := base
				touched := false
				for _, rg := range selected {
					if rg.Contains(r, col) {
						next = patch.apply(base, command.At{Row: r, Col: col, Range: rg})
						touched = true
						break
					}
				}
				text, _ := display(c, next)
				width := 0.0
				for column := area.C0; column <= area.C1; column++ {
					width += sh.Layout.ColWidth(column)
				}
				if width <= 0 {
					return
				}
				sample := FormatMeasureDTO{Text: text, Style: styleDTO(0, next), Width: width}
				if c.Rich != nil {
					for _, run := range c.Rich.Runs {
						font := run.Font.Resolve(base.Font)
						if touched {
							st := base
							st.Font = font
							font = patch.apply(st, command.At{Row: r, Col: col}).Font
						}
						sample.Runs = append(sample.Runs, TextRunDTO{Text: run.Text, Font: FontDTO{Name: font.Name, Size: font.Size, Bold: font.Bold, Italic: font.Italic, Underline: font.Underline, Strike: font.Strike, Color: font.Color}})
					}
				}
				result.Cells = append(result.Cells, sample)
			})
			if err != nil {
				return
			}
			if len(result.Cells) > 0 {
				out = append(out, result)
			}
		}
	})
	return out, err
}
