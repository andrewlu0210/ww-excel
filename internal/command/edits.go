package command

import (
	"errors"

	"ww-excel/internal/model"
)

// MaxFillCells bounds ⌘Enter fills so a whole-sheet selection cannot
// allocate billions of cells.
const MaxFillCells = 1 << 20

var ErrFillTooLarge = errors.New("selection too large to fill")

// EnterText sets one cell from typed text. Returns nil when the cell
// already holds that value.
func EnterText(wb *model.Workbook, sheet, row, col int, text string) Command {
	cell := model.ParseInput(text)
	if wb.Sheets[sheet].Get(row, col) == cell {
		return nil
	}
	return &SetCells{
		Sheet:   sheet,
		Name:    "輸入",
		Changes: []CellChange{{Row: row, Col: col, After: cell}},
	}
}

// FillRanges sets every cell of the ranges to the parsed text (⌘Enter).
func FillRanges(sheet int, ranges []model.Range, text string) (Command, error) {
	total := 0
	for _, rg := range ranges {
		total += rg.Clamp().CellCount()
	}
	if total > MaxFillCells {
		return nil, ErrFillTooLarge
	}

	cell := model.ParseInput(text)
	changes := make([]CellChange, 0, total)
	var area *model.Range
	for _, rg := range ranges {
		rg = rg.Clamp()
		for r := rg.R0; r <= rg.R1; r++ {
			for c := rg.C0; c <= rg.C1; c++ {
				changes = append(changes, CellChange{Row: r, Col: c, After: cell})
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
