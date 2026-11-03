package command

import "ww-excel/internal/model"

// GrowAutomaticRow measures on the frontend but applies after the cell/table
// edit, so its layout snapshot includes any table expansion in the same batch.
func GrowAutomaticRow(sheet, row int, height float64) Command {
	return &automaticRowHeight{sheet: sheet, row: row, height: height}
}

type automaticRowHeight struct {
	sheet, row int
	height     float64
	inner      Command
}

func (c *automaticRowHeight) Label() string { return "列高" }
func (c *automaticRowHeight) Do(wb *model.Workbook) Effect {
	layout := wb.Sheets[c.sheet].Layout
	c.inner = nil
	if _, custom := layout.Rows[c.row]; custom || c.height <= layout.RowHeight(c.row) {
		return Effect{Sheet: c.sheet}
	}
	c.inner = SetSizes(wb, c.sheet, true, []int{c.row}, c.height)
	return c.inner.Do(wb)
}
func (c *automaticRowHeight) Undo(wb *model.Workbook) Effect {
	if c.inner == nil {
		return Effect{Sheet: c.sheet}
	}
	return c.inner.Undo(wb)
}
