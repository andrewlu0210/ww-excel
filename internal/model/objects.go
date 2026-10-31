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
	Shape  *Shape
	Image  *Image
}

const (
	ObjectShape = "shape"
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
	PlotArea *ChartPlotArea
	// Type: column, bar, line, pie, doughnut, scatter, area.
	Type         string
	ValueAxis    *ChartAxis
	CategoryAxis *ChartAxis
	Background   string // Empty uses the default; transparent or an RGB hex color.
	HoleSize     int    // Doughnut inner diameter as a percentage (0 uses 50).
	Title        string
	Series       []ChartSeries
	Legend       string // right, bottom, top, none
	XTitle       string
	YTitle       string
	Stacked      bool
}

type ChartPlotArea struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}
type ChartGradientStop struct {
	Position float64 `json:"position"`
	Color    string  `json:"color"`
}
type ChartGradient struct {
	Angle float64             `json:"angle"`
	Stops []ChartGradientStop `json:"stops"`
}
type ChartShadow struct {
	Color   string  `json:"color"`
	Opacity float64 `json:"opacity"`
	Blur    float64 `json:"blur"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}
type ChartPointStyle struct {
	Gradient   *ChartGradient `json:"gradient,omitempty"`
	Shadow     *ChartShadow   `json:"shadow,omitempty"`
	LineHidden bool           `json:"lineHidden,omitempty"`
}

type ChartAxis struct {
	Format     string   `json:"format"`
	Minimum    *float64 `json:"minimum,omitempty"`
	Maximum    *float64 `json:"maximum,omitempty"`
	MajorUnit  float64  `json:"majorUnit"`
	LineColor  string   `json:"lineColor"`
	LineHidden bool     `json:"lineHidden"`
	Hidden     bool     `json:"hidden"`
	MajorTick  string   `json:"majorTick"`
	GridLines  bool     `json:"gridLines"`
	HideLabels bool     `json:"hideLabels"`
}

type ChartLabels struct {
	Format      string                  `json:"format"`
	ShowValue   bool                    `json:"showValue"`
	ShowPercent bool                    `json:"showPercent"`
	FontSize    float64                 `json:"fontSize"`
	Color       string                  `json:"color"`
	Points      map[int]ChartLabelPoint `json:"points"`
	RawXML      string                  `json:"rawXML,omitempty"`
}

type ChartLabelPoint struct {
	Hidden bool `json:"hidden"`
	Center bool `json:"center"`
}

type ChartSeries struct {
	PointStyles    map[int]ChartPointStyle `json:"pointStyles,omitempty"`
	CategoryLabels []string                `json:"categoryLabels,omitempty"`
	Color          string                  `json:"color,omitempty"`
	PointColors    map[int]string          `json:"pointColors,omitempty"`
	Labels         *ChartLabels            `json:"labels,omitempty"`
	Name           string                  // a reference to the name cell, or "" (or a quoted literal)
	Categories     string                  // x labels (scatter: x values)
	Values         string
}

func (c *Chart) clone() *Chart {
	if c == nil {
		return nil
	}
	out := *c
	if c.PlotArea != nil {
		layout := *c.PlotArea
		out.PlotArea = &layout
	}
	cloneAxis := func(a *ChartAxis) *ChartAxis {
		if a == nil {
			return nil
		}
		b := *a
		if a.Minimum != nil {
			v := *a.Minimum
			b.Minimum = &v
		}
		if a.Maximum != nil {
			v := *a.Maximum
			b.Maximum = &v
		}
		return &b
	}
	out.ValueAxis = cloneAxis(c.ValueAxis)
	out.CategoryAxis = cloneAxis(c.CategoryAxis)
	out.Series = append([]ChartSeries(nil), c.Series...)
	for i, s := range c.Series {
		if s.PointStyles != nil {
			out.Series[i].PointStyles = make(map[int]ChartPointStyle, len(s.PointStyles))
			for k, style := range s.PointStyles {
				if style.Gradient != nil {
					g := *style.Gradient
					g.Stops = append([]ChartGradientStop(nil), g.Stops...)
					style.Gradient = &g
				}
				if style.Shadow != nil {
					sh := *style.Shadow
					style.Shadow = &sh
				}
				out.Series[i].PointStyles[k] = style
			}
		}
		if s.CategoryLabels != nil {
			out.Series[i].CategoryLabels = append([]string{}, s.CategoryLabels...)
		}
		if s.PointColors != nil {
			out.Series[i].PointColors = make(map[int]string, len(s.PointColors))
			for k, v := range s.PointColors {
				out.Series[i].PointColors[k] = v
			}
		}
		if s.Labels != nil {
			labels := *s.Labels
			labels.Points = make(map[int]ChartLabelPoint, len(s.Labels.Points))
			for k, v := range s.Labels.Points {
				labels.Points[k] = v
			}
			out.Series[i].Labels = &labels
		}
	}
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
