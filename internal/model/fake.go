package model

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// FillFake populates a sheet with deterministic sample data for
// performance testing: a label column followed by numeric columns.
// It also drops markers at the far edges of the grid.
func FillFake(s *Sheet, rows, cols int, seed uint64) {
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))

	headers := []string{"Item", "Region", "Qty", "Price", "Amount", "Cost", "Margin", "Score"}
	for c := 0; c < cols; c++ {
		h := fmt.Sprintf("Col %d", c+1)
		if c < len(headers) {
			h = headers[c]
		}
		s.Set(0, c, StringCell(h))
	}

	regions := []string{"台北", "新竹", "台中", "台南", "高雄", "North", "South"}
	for r := 1; r < rows; r++ {
		for c := 0; c < cols; c++ {
			var cell Cell
			switch c {
			case 0:
				cell = StringCell(fmt.Sprintf("Item %06d", r))
			case 1:
				cell = StringCell(regions[rng.IntN(len(regions))])
			default:
				cell = NumberCell(math.Round(rng.Float64()*100000) / 100)
			}
			s.Set(r, c, cell)
		}
	}

	s.Set(MaxRows-1, 0, StringCell("last row"))
	s.Set(0, MaxCols-1, StringCell("last col"))
	s.Set(MaxRows-1, MaxCols-1, StringCell("XFD1048576"))
}
