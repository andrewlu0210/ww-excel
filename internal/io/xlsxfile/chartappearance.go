package xlsxfile

import (
	"math"
	"strconv"
	"ww-excel/internal/model"
)

type rawChartLayout struct {
	X rawAxisNumber `xml:"x"`
	Y rawAxisNumber `xml:"y"`
	W rawAxisNumber `xml:"w"`
	H rawAxisNumber `xml:"h"`
}

func (l *rawChartLayout) model() *model.ChartPlotArea {
	if l == nil || l.W.Val <= 0 || l.H.Val <= 0 {
		return nil
	}
	for _, v := range []float64{l.X.Val, l.Y.Val, l.W.Val, l.H.Val} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil
		}
	}
	return &model.ChartPlotArea{X: l.X.Val, Y: l.Y.Val, W: l.W.Val, H: l.H.Val}
}

type rawChartAppearance struct {
	Fill     drawingFill `xml:"solidFill"`
	Gradient *struct {
		Stops []struct {
			Position float64 `xml:"pos,attr"`
			drawingFill
		} `xml:"gsLst>gs"`
		Line struct {
			Angle float64 `xml:"ang,attr"`
		} `xml:"lin"`
	} `xml:"gradFill"`
	Line struct {
		Hidden *struct{} `xml:"noFill"`
	} `xml:"ln"`
	Shadow *struct {
		Blur      float64 `xml:"blurRad,attr"`
		Distance  float64 `xml:"dist,attr"`
		Direction float64 `xml:"dir,attr"`
		drawingFill
	} `xml:"effectLst>outerShdw"`
}

func (p rawChartAppearance) model(theme map[string]string) (model.ChartPointStyle, bool) {
	out := model.ChartPointStyle{LineHidden: p.Line.Hidden != nil}
	if p.Gradient != nil {
		gradient := &model.ChartGradient{Angle: p.Gradient.Line.Angle / 60000}
		for _, stop := range p.Gradient.Stops {
			color := stop.drawingFill.color(theme)
			if _, err := strconv.ParseUint(color, 16, 32); err != nil || color == "" || math.IsNaN(stop.Position) || math.IsInf(stop.Position, 0) {
				continue
			}
			gradient.Stops = append(gradient.Stops, model.ChartGradientStop{Position: math.Max(0, math.Min(1, stop.Position/100000)), Color: color})
		}
		if len(gradient.Stops) > 0 {
			out.Gradient = gradient
		}
	}
	if shadow := p.Shadow; shadow != nil {
		color := shadow.drawingFill.color(theme)
		opacity := 1.0
		c := shadow.drawingFill.RGB
		if c == nil {
			c = shadow.drawingFill.Scheme
		}
		if c != nil && c.Alpha != nil {
			opacity = math.Max(0, math.Min(1, c.Alpha.Val/100000))
		}
		angle := shadow.Direction / 60000 * math.Pi / 180
		out.Shadow = &model.ChartShadow{Color: color, Opacity: opacity, Blur: math.Max(0, math.Min(100, shadow.Blur/emuPerPx)), X: math.Cos(angle) * shadow.Distance / emuPerPx, Y: math.Sin(angle) * shadow.Distance / emuPerPx}
	}
	return out, out.Gradient != nil || out.Shadow != nil || out.LineHidden
}
