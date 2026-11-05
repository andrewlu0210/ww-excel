package xlsxfile

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/model"
)

func excelAxis(a *model.ChartAxis) excelize.ChartAxis {
	if a == nil {
		return excelize.ChartAxis{}
	}
	return excelize.ChartAxis{None: a.Hidden, MajorGridLines: a.GridLines, Minimum: a.Minimum, Maximum: a.Maximum, MajorUnit: a.MajorUnit, NumFmt: excelize.ChartNumFmt{CustomNumFmt: a.Format}, Font: excelize.Font{Family: a.FontName, Size: a.FontSize}}
}

// Excelize writes each chart type/axis pair as a plot within one chart part.
func (w *writer) addComboChart(sh *model.Sheet, o model.Object, cell string, format excelize.GraphicOptions) error {
	c := o.Chart
	type group struct {
		typ       string
		secondary bool
		series    []model.ChartSeries
	}
	var groups []group
	for _, secondary := range []bool{false, true} {
		for _, s := range c.Series {
			if s.Secondary != secondary {
				continue
			}
			if s.Type != "column" && s.Type != "line" {
				return fmt.Errorf("不支援的圖表類型「%s」", s.Type)
			}
			index := -1
			for i, g := range groups {
				if g.typ == s.Type && g.secondary == secondary {
					index = i
					break
				}
			}
			if index < 0 {
				groups = append(groups, group{typ: s.Type, secondary: secondary})
				index = len(groups) - 1
			}
			groups[index].series = append(groups[index].series, s)
		}
	}
	if len(groups) == 0 {
		return fmt.Errorf("圖表至少要有一個數列")
	}
	var charts []*excelize.Chart
	ordered := *c
	ordered.Series = nil
	for _, g := range groups {
		typ := excelize.Col
		if c.Stacked {
			typ = excelize.ColStacked
		}
		if g.typ == "line" {
			typ = excelize.Line
		}
		xc := &excelize.Chart{Type: typ, Format: format, Dimension: chartDrawingDimension(o, format), Legend: excelize.ChartLegend{Position: legendPos(c.Legend)}}
		axis, category := c.ValueAxis, c.CategoryAxis
		if g.secondary {
			axis, category = c.SecondaryAxis, c.SecondaryCategoryAxis
		}
		xc.YAxis = excelAxis(axis)
		xc.YAxis.Secondary = g.secondary
		xc.XAxis = excelAxis(category)
		if c.GapWidth != nil {
			gap := uint(max(0, min(500, *c.GapWidth)))
			xc.GapWidth = &gap
		}
		if c.Background == "transparent" {
			xc.Fill = excelize.Fill{Type: "pattern"}
		} else if c.Background != "" {
			xc.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{c.Background}}
		}
		if c.Border != "" {
			xc.Border = excelize.LineOptions{Width: .75, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{c.Border}}}
		}
		if c.Title != "" {
			xc.Title = excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: c.Title}}, Font: &excelize.Font{Family: c.FontName, Size: c.TitleSize}}
		}
		for _, s := range g.series {
			xs := excelize.ChartSeries{Name: s.Name, Categories: s.Categories, Values: s.Values, Marker: excelize.ChartMarker{Symbol: s.Marker}}
			if s.Color != "" {
				fill := excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{s.Color}}
				xs.Fill = fill
				xs.Line.Fill = fill
			}
			if s.LineWidth > 0 {
				xs.Line.Width = s.LineWidth * 72 / 96
			}
			for index, color := range s.PointColors {
				xs.DataPoint = append(xs.DataPoint, excelize.ChartDataPoint{Index: index, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{color}}})
			}
			xc.Series = append(xc.Series, xs)
			ordered.Series = append(ordered.Series, s)
		}
		charts = append(charts, xc)
	}
	before := map[string]bool{}
	w.f.Pkg.Range(func(k, v any) bool { before[k.(string)] = true; return true })
	if err := w.f.AddChart(sh.Name, cell, charts[0], charts[1:]...); err != nil {
		return err
	}
	var writeErr error
	w.f.Pkg.Range(func(k, v any) bool {
		name := k.(string)
		if !before[name] && strings.HasPrefix(name, "xl/charts/chart") && strings.Count(name, "/") == 2 && strings.HasSuffix(name, ".xml") {
			data := restoreChartAppearance(restoreChartCategories(restoreChartLabels(v.([]byte), ordered.Series), ordered.Series), &ordered)
			if err := w.preserveChartPart(name, c, restoreComboAxes(data, &ordered)); err != nil {
				writeErr = err
				return false
			}
		}
		return true
	})
	return writeErr
}
