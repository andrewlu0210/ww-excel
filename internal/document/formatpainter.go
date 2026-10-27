package document

import (
	"errors"

	"ww-excel/internal/command"
	"ww-excel/internal/model"
)

// FormatSample captures only effective cell styles, independent of later edits.
// Style IDs remain valid for the lifetime of the source workbook.
type FormatSample struct {
	workbook   *model.Workbook
	rows, cols int
	uniform    bool
	styles     []uint32
}

func (d *Document) CaptureFormats(sheet int, area model.Range) (*FormatSample, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	area = area.Clamp()
	if area.CellCount() > command.MaxFillCells {
		return nil, ErrClipTooLarge
	}
	sample := &FormatSample{workbook: d.wb, rows: area.R1 - area.R0 + 1, cols: area.C1 - area.C0 + 1, styles: make([]uint32, 0, area.CellCount())}
	sh, table := d.wb.Sheets[sheet], d.wb.StyleTable()
	ids := map[uint32]uint32{}
	for r := area.R0; r <= area.R1; r++ {
		for c := area.C0; c <= area.C1; c++ {
			id := sh.EffectiveStyle(r, c)
			explicit, ok := ids[id]
			if !ok {
				explicit = table.InternExplicit(table.Get(id))
				ids[id] = explicit
			}
			sample.styles = append(sample.styles, explicit)
		}
	}
	// Uniform sources can format entire rows/columns without dense writes.
	uniform := true
	for _, id := range sample.styles {
		if id != sample.styles[0] {
			uniform = false
			break
		}
	}
	if uniform {
		sample.uniform = true
		sample.styles = []uint32{sample.styles[0]}
	}
	return sample, nil
}

func (d *Document) PaintFormats(sample *FormatSample, sheet int, ranges []model.Range) (command.Command, error) {
	if sample == nil || sample.workbook != d.wb {
		return nil, errors.New("請先選取來源並啟用複製格式")
	}
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil, errors.New("無效的工作表")
	}
	targets := make([]model.Range, len(ranges))
	for i, r := range ranges {
		r = r.Clamp()
		if r.CellCount() == 1 {
			r.R1, r.C1 = r.R0+sample.rows-1, r.C0+sample.cols-1
		}
		if r.R1 >= model.MaxRows || r.C1 >= model.MaxCols {
			return nil, errors.New("無法貼上：貼上的區域超出工作表")
		}
		// A varying pattern cannot be represented by a single row/column style.
		if !sample.uniform && ((r.R0 == 0 && r.R1 == model.MaxRows-1 && sample.rows > 1) || (r.C0 == 0 && r.C1 == model.MaxCols-1 && sample.cols > 1)) {
			return nil, ErrClipTooLarge
		}
		targets[i] = r
	}
	return command.ApplyStyle(d.wb, sheet, targets, "複製格式", func(_ model.Style, at command.At) model.Style {
		if sample.uniform {
			return d.wb.StyleTable().Get(sample.styles[0])
		}
		r, c := max(0, at.Row-at.Range.R0)%sample.rows, max(0, at.Col-at.Range.C0)%sample.cols
		return d.wb.StyleTable().Get(sample.styles[r*sample.cols+c])
	})
}
