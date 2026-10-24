// Package printing lays a sheet out on pages and renders them to PDF.
// It does not depend on Wails; the service supplies cell display text
// and chart pictures.
package printing

import (
	"errors"

	"ww-excel/internal/model"
)

// Points per inch, and points per screen pixel (96 dpi).
const (
	ptPerInch = 72.0
	ptPerPx   = 0.75
	// Width of the row number column and height of the column letter row
	// when headings are printed, in points before scaling.
	headingW = 30.0
	headingH = 15.0
)

// Paper sizes in points (portrait).
var papers = map[string][2]float64{
	"A4":     {595.28, 841.89},
	"A3":     {841.89, 1190.55},
	"B5":     {515.91, 728.5},
	"Letter": {612, 792},
	"Legal":  {612, 1008},
}

// Page is one printed page: a block of rows and columns.
type Page struct {
	Number int
	Rows   [2]int // inclusive
	Cols   [2]int
}

// Plan is the page layout of a sheet.
type Plan struct {
	Setup          model.PageSetup
	PaperW, PaperH float64 // points, after orientation
	Scale          float64
	Area           model.Range
	Pages          []Page
	// Printable box (inside margins), in points.
	X0, Y0, W, H float64
}

var ErrNothing = errors.New("沒有可以列印的內容")

// Paginate splits the print area into pages. Pages go down, then over,
// like Excel's default page order.
func Paginate(sh *model.Sheet, ps model.PageSetup) (*Plan, error) {
	if ps.Paper == "" {
		ps = model.DefaultPageSetup()
	}
	size, ok := papers[ps.Paper]
	if !ok {
		size = papers["A4"]
	}
	p := &Plan{Setup: ps, PaperW: size[0], PaperH: size[1]}
	if ps.Landscape {
		p.PaperW, p.PaperH = p.PaperH, p.PaperW
	}
	p.X0, p.Y0 = ps.Left*ptPerInch, ps.Top*ptPerInch
	p.W = p.PaperW - (ps.Left+ps.Right)*ptPerInch
	p.H = p.PaperH - (ps.Top+ps.Bottom)*ptPerInch
	if p.W < 72 || p.H < 72 {
		return nil, errors.New("邊界太大，頁面上沒有可列印的空間")
	}

	area, ok := printArea(sh, ps)
	if !ok {
		return nil, ErrNothing
	}
	p.Area = area
	l := &sh.Layout
	colPt := func(c int) float64 { return l.ColWidth(c) * ptPerPx }
	rowPt := func(r int) float64 { return l.RowHeight(r) * ptPerPx }
	extraW, extraH := 0.0, 0.0
	if ps.Headings {
		extraW, extraH = headingW, headingH
	}

	p.Scale = float64(max(ps.Scale, 10)) / 100
	if ps.Scale == 0 {
		p.Scale = 1
	}
	if ps.FitWide > 0 || ps.FitTall > 0 {
		totalW, totalH := 0.0, 0.0
		for c := area.C0; c <= area.C1; c++ {
			totalW += colPt(c)
		}
		for r := area.R0; r <= area.R1; r++ {
			totalH += rowPt(r)
		}
		p.Scale = 1
		if ps.FitWide > 0 && totalW > 0 {
			p.Scale = min(p.Scale, float64(ps.FitWide)*(p.W-extraW)/totalW)
		}
		if ps.FitTall > 0 && totalH > 0 {
			p.Scale = min(p.Scale, float64(ps.FitTall)*(p.H-extraH)/totalH)
		}
		p.Scale = max(p.Scale, 0.1)
	}

	colBands := bands(area.C0, area.C1, func(c int) float64 { return colPt(c) * p.Scale }, p.W-extraW*p.Scale)
	rowBands := bands(area.R0, area.R1, func(r int) float64 { return rowPt(r) * p.Scale }, p.H-extraH*p.Scale)
	for _, cb := range colBands {
		for _, rb := range rowBands {
			p.Pages = append(p.Pages, Page{Number: len(p.Pages) + 1, Rows: rb, Cols: cb})
		}
	}
	return p, nil
}

// bands splits [lo, hi] into runs that fit in limit. A single item
// wider than the page gets a page of its own.
func bands(lo, hi int, size func(int) float64, limit float64) [][2]int {
	var out [][2]int
	start, used := lo, 0.0
	for i := lo; i <= hi; i++ {
		s := size(i)
		if used > 0 && used+s > limit {
			out = append(out, [2]int{start, i - 1})
			start, used = i, 0
		}
		used += s
	}
	return append(out, [2]int{start, hi})
}

// printArea is the print area, else the used range extended to cover
// floating objects. ok is false for an empty sheet.
func printArea(sh *model.Sheet, ps model.PageSetup) (model.Range, bool) {
	if ps.PrintArea != nil {
		return ps.PrintArea.Clamp(), true
	}
	rows, cols := sh.UsedRange()
	l := &sh.Layout
	for _, o := range l.Objects {
		r, c := ObjectEnd(l, o)
		rows, cols = max(rows, r+1), max(cols, c+1)
	}
	for _, m := range l.Merges {
		rows, cols = max(rows, m.R1+1), max(cols, m.C1+1)
	}
	if rows == 0 || cols == 0 {
		return model.Range{}, false
	}
	return model.Range{R1: rows - 1, C1: cols - 1}, true
}

// ObjectEnd is the cell under an object's bottom-right corner.
func ObjectEnd(l *model.Layout, o model.Object) (row, col int) {
	row, col = o.Anchor.Row, o.Anchor.Col
	h := o.Anchor.DY + o.H
	for h > l.RowHeight(row) && row < model.MaxRows-1 {
		h -= l.RowHeight(row)
		row++
	}
	w := o.Anchor.DX + o.W
	for w > l.ColWidth(col) && col < model.MaxCols-1 {
		w -= l.ColWidth(col)
		col++
	}
	return row, col
}
