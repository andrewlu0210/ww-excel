package command

import (
	"errors"

	"ww-excel/internal/model"
)

// MaxFillCells bounds ⌘Enter fills so a whole-sheet selection cannot
// allocate billions of cells.
const MaxFillCells = 1 << 20

var ErrFillTooLarge = errors.New("selection too large to fill")

// SetCell writes one cell (typed input), keeping its format. numFmt is a
// format detected from the input (e.g. "0%" for "12%"); it is applied
// only when the cell's format is General. wrap turns on text wrapping
// (typing a line break does that in Excel). Returns nil when nothing
// would change.
func SetCell(wb *model.Workbook, sheet, row, col int, cell model.Cell, numFmt string, wrap bool) Command {
	sh := wb.Sheets[sheet]
	old := sh.Get(row, col)
	cell.S = inputStyle(wb, sh, row, col, numFmt, wrap)
	if old.SameInput(cell) && old.S == cell.S {
		return nil
	}
	return &SetCells{
		Sheet:   sheet,
		Name:    "輸入",
		Changes: []CellChange{{Row: row, Col: col, After: cell}},
	}
}

func inputStyle(wb *model.Workbook, sh *model.Sheet, row, col int, numFmt string, wrap bool) uint32 {
	id := sh.EffectiveStyle(row, col)
	if numFmt == "" && !wrap {
		return id
	}
	st := wb.StyleTable().Get(id)
	changed := false
	if numFmt != "" && st.NumFmt == "" {
		st.NumFmt = numFmt
		changed = true
	}
	if wrap && !st.Align.Wrap {
		st.Align.Wrap = true
		changed = true
	}
	if !changed {
		return id
	}
	return wb.StyleTable().Intern(st)
}

// EnterText is SetCell for plain values (no formula compilation).
func EnterText(wb *model.Workbook, sheet, row, col int, text string) Command {
	return SetCell(wb, sheet, row, col, model.ParseInput(text), "", false)
}

// FillRanges sets every cell of the ranges (⌘Enter); cellAt produces the
// cell for each position, e.g. a formula adjusted to that cell.
func FillRanges(wb *model.Workbook, sheet int, ranges []model.Range, cellAt func(row, col int) model.Cell) (Command, error) {
	total := 0
	for _, rg := range ranges {
		total += rg.Clamp().CellCount()
	}
	if total > MaxFillCells {
		return nil, ErrFillTooLarge
	}

	changes := []CellChange{}
	var area *model.Range
	for _, rg := range ranges {
		rg = rg.Clamp()
		for r := rg.R0; r <= rg.R1; r++ {
			for c := rg.C0; c <= rg.C1; c++ {
				cell := cellAt(r, c)
				cell.S = sheetStyle(wb, sheet, r, c, cell.S)
				changes = append(changes, CellChange{Row: r, Col: c, After: cell})
			}
		}
		area = union(area, rg)
	}
	return &SetCells{Sheet: sheet, Name: "填滿", Changes: changes, Area: area}, nil
}

// ClearMode selects what ClearRanges removes.
type ClearMode int

const (
	ClearContents ClearMode = iota // Delete key: values and formulas
	ClearFormats                   // formats only
	ClearAll
)

// ClearRanges empties the ranges. Only existing cells are recorded, so
// clearing whole columns stays cheap. Returns nil if nothing changes.
func ClearRanges(wb *model.Workbook, sheet int, ranges []model.Range, mode ClearMode) Command {
	sh := wb.Sheets[sheet]
	var changes []CellChange
	seen := make(map[CellRef]bool)
	for _, rg := range ranges {
		sh.EachInRange(rg, func(row, col int, c model.Cell) {
			ref := CellRef{row, col}
			if seen[ref] {
				return
			}
			seen[ref] = true
			after := c
			switch mode {
			case ClearContents:
				after = model.Cell{S: c.S}
			case ClearFormats:
				after.S = 0
			case ClearAll:
				after = model.Cell{}
			}
			if after.SameInput(c) && after.S == c.S {
				return
			}
			changes = append(changes, CellChange{Row: row, Col: col, After: after})
		})
	}
	if len(changes) == 0 {
		return nil
	}
	names := map[ClearMode]string{ClearContents: "清除內容", ClearFormats: "清除格式", ClearAll: "全部清除"}
	return &SetCells{Sheet: sheet, Name: names[mode], Changes: changes}
}

// sheetStyle keeps an explicit style, else inherits the row/column one.
func sheetStyle(wb *model.Workbook, sheet, row, col int, explicit uint32) uint32 {
	if explicit != 0 {
		return explicit
	}
	return wb.Sheets[sheet].EffectiveStyle(row, col)
}

func union(a *model.Range, b model.Range) *model.Range {
	if a == nil {
		return &b
	}
	a.R0, a.C0 = min(a.R0, b.R0), min(a.C0, b.C0)
	a.R1, a.C1 = max(a.R1, b.R1), max(a.C1, b.C1)
	return a
}
