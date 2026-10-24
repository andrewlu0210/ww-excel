package model

// Floating objects (charts and pictures) and page setup, kept in the
// sheet layout so undo, row/column shifts and in-place saves handle
// them like the rest of the layout.

// Object is a chart or picture floating over the grid. Its top-left
// corner is anchored to a cell plus an offset in pixels, so it moves
// with that cell when rows or columns are inserted.
type Object struct {
	ID     int // unique within the sheet
	Kind   string
	Anchor Anchor
	W, H   float64 // size in pixels at 100%
	Chart  *Chart
	Image  *Image
}

const (
	ObjectChart = "chart"
	ObjectImage = "image"
)

type Anchor struct {
	Row, Col int
	DX, DY   float64
}

// Image is an embedded picture. Data is shared between layout clones
// and never modified.
type Image struct {
	Data []byte
	Ext  string // ".png", ".jpg", ".gif"
}

// Chart is a chart linked to cells. Series references are formulas
// without "=", usually absolute references with the sheet name.
type Chart struct {
	// Type: column, bar, line, pie, scatter, area.
	Type    string
	Title   string
	Series  []ChartSeries
	Legend  string // right, bottom, top, none
	XTitle  string
	YTitle  string
	Stacked bool
}

type ChartSeries struct {
	Name       string // a reference to the name cell, or "" (or a quoted literal)
	Categories string // x labels (scatter: x values)
	Values     string
}

func (c *Chart) clone() *Chart {
	if c == nil {
		return nil
	}
	out := *c
	out.Series = append([]ChartSeries(nil), c.Series...)
	return &out
}

func cloneObjects(objs []Object) []Object {
	out := make([]Object, len(objs))
	for i, o := range objs {
		o.Chart = o.Chart.clone()
		out[i] = o
	}
	return out
}

// NextObjectID returns an unused object ID for the sheet.
func (l *Layout) NextObjectID() int {
	id := 1
	for _, o := range l.Objects {
		id = max(id, o.ID+1)
	}
	return id
}

// ObjectByID finds an object.
func (l *Layout) ObjectByID(id int) *Object {
	for i := range l.Objects {
		if l.Objects[i].ID == id {
			return &l.Objects[i]
		}
	}
	return nil
}

// PageSetup holds the print settings of a sheet (版面設定).
type PageSetup struct {
	Paper     string // A4, A3, B5, Letter, Legal
	Landscape bool
	// Margins in inches.
	Top, Bottom, Left, Right, Header, Footer float64
	// Scale in percent (10-400). FitWide/FitTall > 0 fit the print area
	// to that many pages instead (0 = no limit in that direction).
	Scale            int
	FitWide, FitTall int
	PrintArea        *Range
	Gridlines        bool
	Headings         bool
	CenterH, CenterV bool
	// Header and footer text (center section). Codes: &P page, &N pages,
	// &D date, &T time, &F file name, &A sheet name.
	HeaderText, FooterText string
}

// DefaultPageSetup matches Excel's defaults: A4 portrait, normal
// margins, page number in the footer.
func DefaultPageSetup() PageSetup {
	return PageSetup{Paper: "A4", Top: 0.75, Bottom: 0.75, Left: 0.7, Right: 0.7, Header: 0.3, Footer: 0.3, Scale: 100, FooterText: "第 &P 頁，共 &N 頁"}
}
