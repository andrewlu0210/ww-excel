package command

import (
	"errors"

	"ww-excel/internal/model"
)

// ErrMergeOverlap means a merge would partially cover another merge.
var ErrMergeOverlap = errors.New("無法合併部分重疊的合併儲存格")

// At tells a style patch where it applies: a cell, or a whole row
// (Col = -1) or column (Row = -1), inside Range (for edge-aware patches
// such as outside borders).
type At struct {
	Row, Col int
	Range    model.Range
}

// Patch changes a style at a position.
type Patch func(st model.Style, at At) model.Style

// ApplyStyle changes the format of ranges with patch. Whole columns and
// rows update the column/row styles (so A:A does not create a million
// cells) plus the existing cells inside; other ranges update every cell,
// including empty ones, like Excel.
func ApplyStyle(wb *model.Workbook, sheet int, ranges []model.Range, name string, patch Patch) (Command, error) {
	sh := wb.Sheets[sheet]
	styles := wb.StyleTable()
	var cur model.Range
	restyleAt := func(id uint32, row, col int) uint32 {
		return styles.Intern(patch(styles.Get(id), At{Row: row, Col: col, Range: cur}))
	}

	layout := sh.Layout.Clone()
	layoutChanged := false
	cells := &SetCells{Sheet: sheet, Name: name}
	seen := map[CellRef]bool{}
	touch := func(row, col int) {
		ref := CellRef{row, col}
		if seen[ref] {
			return
		}
		seen[ref] = true
		c := sh.Get(row, col)
		id := c.S
		if id == 0 {
			id = sh.Layout.StyleAt(row, col)
		}
		if c.Rich != nil {
			base := styles.Get(id)
			rich := &model.RichText{Runs: append([]model.TextRun(nil), c.Rich.Runs...)}
			changed := false
			for i, run := range rich.Runs {
				before := run.Font.Resolve(base.Font)
				st := base
				st.Font = before
				next := patch(st, At{Row: row, Col: col, Range: cur}).Font
				if next != before {
					rich.Runs[i].Font = model.RichFontOf(next)
					changed = true
				}
			}
			if changed {
				c.Rich = rich
			}
		}
		c.S = restyleAt(id, row, col)
		cells.Changes = append(cells.Changes, CellChange{Row: row, Col: col, After: c})
	}

	total := 0
	var area *model.Range
	for _, rg := range ranges {
		rg = rg.Clamp()
		cur = rg
		area = union(area, rg)
		fullCols := rg.R0 == 0 && rg.R1 == model.MaxRows-1
		fullRows := rg.C0 == 0 && rg.C1 == model.MaxCols-1
		switch {
		case fullCols:
			for c := rg.C0; c <= rg.C1; c++ {
				layout.ColStyles[c] = restyleAt(sh.Layout.ColStyles[c], -1, c)
			}
			// Rows with their own style intersect these columns: their
			// cells get explicit styles so the column format shows.
			for r := range sh.Layout.RowStyles {
				if fullRows {
					layout.RowStyles[r] = restyleAt(sh.Layout.RowStyles[r], r, -1)
					continue
				}
				for c := rg.C0; c <= rg.C1; c++ {
					touch(r, c)
				}
			}
			layoutChanged = true
			sh.EachInRange(rg, func(row, col int, _ model.Cell) { touch(row, col) })
		case fullRows:
			for r := rg.R0; r <= rg.R1; r++ {
				layout.RowStyles[r] = restyleAt(sh.Layout.StyleAt(r, -1), r, -1)
			}
			layoutChanged = true
			sh.EachInRange(rg, func(row, col int, _ model.Cell) { touch(row, col) })
		default:
			total += rg.CellCount()
			if total > MaxFillCells {
				return nil, ErrFillTooLarge
			}
			for r := rg.R0; r <= rg.R1; r++ {
				for c := rg.C0; c <= rg.C1; c++ {
					touch(r, c)
				}
			}
		}
	}
	cells.Area = area
	if !layoutChanged {
		if len(cells.Changes) == 0 {
			return nil, nil
		}
		return cells, nil
	}
	return &Layout{Sheet: sheet, Name: name, After: layout, Cells: cells, Area: area}, nil
}

// SetSizes sets row heights (rows) or column widths of the given
// indices; px < 0 restores the default size.
func SetSizes(wb *model.Workbook, sheet int, rows bool, indices []int, px float64) Command {
	sh := wb.Sheets[sheet]
	layout := sh.Layout.Clone()
	target := layout.Cols
	name := "欄寬"
	if rows {
		target = layout.Rows
		name = "列高"
	}
	for _, i := range indices {
		if px < 0 {
			delete(target, i)
			continue
		}
		target[i] = model.Size{Px: px}
	}
	return &Layout{Sheet: sheet, Name: name, After: layout}
}

// Merge merges rg. Values other than the top-left one are removed (Excel
// warns about that first). center also centers the content ("跨欄置中").
func Merge(wb *model.Workbook, sheet int, rg model.Range, center bool) (Command, error) {
	sh := wb.Sheets[sheet]
	rg = rg.Clamp()
	if rg.R0 == rg.R1 && rg.C0 == rg.C1 {
		return nil, nil
	}
	layout := sh.Layout.Clone()
	var kept []model.Range
	for _, m := range layout.Merges {
		if !m.Overlaps(rg) {
			kept = append(kept, m)
			continue
		}
		inside := m.R0 >= rg.R0 && m.R1 <= rg.R1 && m.C0 >= rg.C0 && m.C1 <= rg.C1
		if !inside {
			return nil, ErrMergeOverlap
		}
	}
	layout.Merges = append(kept, rg)
	layout.SortMerges()

	cells := &SetCells{Sheet: sheet, Name: "合併儲存格"}
	sh.EachInRange(rg, func(row, col int, c model.Cell) {
		if row == rg.R0 && col == rg.C0 || !c.HasContent() {
			return
		}
		cells.Changes = append(cells.Changes, CellChange{Row: row, Col: col, After: model.Cell{S: c.S}})
	})
	if center {
		styles := wb.StyleTable()
		c := sh.Get(rg.R0, rg.C0)
		st := styles.Get(sh.EffectiveStyle(rg.R0, rg.C0))
		st.Align.H = "center"
		c.S = styles.Intern(st)
		cells.Changes = append(cells.Changes, CellChange{Row: rg.R0, Col: rg.C0, After: c})
	}
	name := "合併儲存格"
	if center {
		name = "跨欄置中"
	}
	cells.Name = name
	return &Layout{Sheet: sheet, Name: name, After: layout, Cells: cells, Area: &rg}, nil
}

// Unmerge removes merges overlapping the ranges.
func Unmerge(wb *model.Workbook, sheet int, ranges []model.Range) Command {
	sh := wb.Sheets[sheet]
	layout := sh.Layout.Clone()
	var kept []model.Range
	removed := false
	for _, m := range layout.Merges {
		hit := false
		for _, rg := range ranges {
			if m.Overlaps(rg) {
				hit = true
				break
			}
		}
		if hit {
			removed = true
		} else {
			kept = append(kept, m)
		}
	}
	if !removed {
		return nil
	}
	layout.Merges = kept
	return &Layout{Sheet: sheet, Name: "取消合併儲存格", After: layout}
}
