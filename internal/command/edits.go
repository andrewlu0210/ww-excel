package command

import (
	"errors"

	"ww-excel/internal/model"
)

// MaxFillCells bounds ⌘Enter fills so a whole-sheet selection cannot
// allocate billions of cells.
const MaxFillCells = 1 << 20

var ErrFillTooLarge = errors.New("selection too large to fill")

// SetCell writes one cell (typed input). Returns nil when the cell
// already holds the same input.
func SetCell(wb *model.Workbook, sheet, row, col int, cell model.Cell) Command {
	if wb.Sheets[sheet].Get(row, col).SameInput(cell) {
		return nil
	}
	return &SetCells{
		Sheet:   sheet,
		Name:    "輸入",
		Changes: []CellChange{{Row: row, Col: col, After: cell}},
	}
}

// EnterText is SetCell for plain values (no formula compilation).
func EnterText(wb *model.Workbook, sheet, row, col int, text string) Command {
	return SetCell(wb, sheet, row, col, model.ParseInput(text))
}

// FillRanges sets every cell of the ranges (⌘Enter); cellAt produces the
// cell for each position, e.g. a formula adjusted to that cell.
func FillRanges(sheet int, ranges []model.Range, cellAt func(row, col int) model.Cell) (Command, error) {
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
				changes = append(changes, CellChange{Row: r, Col: c, After: cellAt(r, c)})
			}
		}
		area = union(area, rg)
	}
	return &SetCells{Sheet: sheet, Name: "填滿", Changes: changes, Area: area}, nil
}

// ClearRanges empties the ranges. Only non-empty cells are recorded, so
// clearing whole columns stays cheap. Returns nil if nothing to clear.
func ClearRanges(wb *model.Workbook, sheet int, ranges []model.Range) Command {
	sh := wb.Sheets[sheet]
	var changes []CellChange
	seen := make(map[CellRef]bool)
	for _, rg := range ranges {
		sh.EachInRange(rg, func(row, col int, _ model.Cell) {
			ref := CellRef{row, col}
			if !seen[ref] {
				seen[ref] = true
				changes = append(changes, CellChange{Row: row, Col: col})
			}
		})
	}
	if len(changes) == 0 {
		return nil
	}
	return &SetCells{Sheet: sheet, Name: "清除內容", Changes: changes}
}

func union(a *model.Range, b model.Range) *model.Range {
	if a == nil {
		return &b
	}
	a.R0, a.C0 = min(a.R0, b.R0), min(a.C0, b.C0)
	a.R1, a.C1 = max(a.R1, b.R1), max(a.C1, b.C1)
	return a
}
