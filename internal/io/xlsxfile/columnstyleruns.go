package xlsxfile

import "sort"

type columnStyleRun struct {
	first, last int
	style       uint32
}

// Excelize repeatedly flattens its column list on SetColStyle. Coalesce equal
// adjacent styles so a full-width template does not trigger quadratic work.
func columnStyleRuns(old, current map[int]uint32) []columnStyleRun {
	keys := union(old, current)
	columns := make([]int, 0, len(keys))
	for col := range keys {
		columns = append(columns, col)
	}
	sort.Ints(columns)
	runs := []columnStyleRun{}
	for _, col := range columns {
		style := current[col]
		if n := len(runs); n > 0 && runs[n-1].last+1 == col && runs[n-1].style == style {
			runs[n-1].last = col
		} else {
			runs = append(runs, columnStyleRun{col, col, style})
		}
	}
	return runs
}
