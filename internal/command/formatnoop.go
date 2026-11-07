package command

import (
	"maps"
	"ww-excel/internal/model"
)

// PruneFormat removes changes whose final style/content is already present.
// Compare against the proposed layout so necessary inheritance overrides stay.
func PruneFormat(wb *model.Workbook, cmd Command) Command {
	if cmd == nil {
		return nil
	}
	var cells *SetCells
	var layout *Layout
	switch c := cmd.(type) {
	case *SetCells:
		cells = c
	case *Layout:
		layout = c
		cells = c.Cells
	default:
		return cmd
	}
	sh := wb.Sheets[cells.Sheet]
	afterLayout := sh.Layout
	if layout != nil {
		afterLayout = layout.After
	}
	kept := cells.Changes[:0]
	for _, ch := range cells.Changes {
		before := sh.Get(ch.Row, ch.Col)
		beforeID := before.S
		if beforeID == 0 {
			beforeID = afterLayout.StyleAt(ch.Row, ch.Col)
		}
		afterID := ch.After.S
		if afterID == 0 {
			afterID = afterLayout.StyleAt(ch.Row, ch.Col)
		}
		if before.Value() == ch.After.Value() && before.Rich == ch.After.Rich && wb.Styles.Get(beforeID) == wb.Styles.Get(afterID) {
			continue
		}
		kept = append(kept, ch)
	}
	cells.Changes = kept
	if layout != nil && (!maps.EqualFunc(sh.Layout.RowStyles, layout.After.RowStyles, func(a, b uint32) bool { return wb.Styles.Get(a) == wb.Styles.Get(b) }) || !maps.EqualFunc(sh.Layout.ColStyles, layout.After.ColStyles, func(a, b uint32) bool { return wb.Styles.Get(a) == wb.Styles.Get(b) })) {
		return layout
	}
	if len(kept) == 0 {
		return nil
	}
	return cells
}
