package xlsxfile

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"ww-excel/internal/model"
)

// Preserve point effects and manual plot layout omitted by Excelize's chart API.
func restoreChartAppearance(data []byte, c *model.Chart) []byte {
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	d := xml.NewDecoder(bytes.NewReader(data))
	depth, seriesDepth, seriesIndex, pointStart, pointDepth, pointIndex, plotDepth, layoutStart, layoutDepth := 0, 0, -1, -1, 0, -1, 0, -1, 0
	seen := map[int]bool{}
	layoutSeen := false
	insertAt := -1
	for {
		start := int(d.InputOffset())
		token, err := d.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			switch {
			case t.Name.Local == "plotArea":
				plotDepth = depth
			case plotDepth > 0 && depth == plotDepth+1 && t.Name.Local == "layout":
				layoutStart = start
				layoutDepth = depth
				layoutSeen = true
			case t.Name.Local == "ser":
				seriesDepth = depth
				seriesIndex++
				seen = map[int]bool{}
				insertAt = -1
			case seriesDepth > 0 && depth == seriesDepth+1 && t.Name.Local == "dPt":
				pointStart = start
				pointDepth = depth
				pointIndex = -1
			case pointStart >= 0 && depth == pointDepth+1 && t.Name.Local == "idx":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						fmt.Sscan(a.Value, &pointIndex)
					}
				}
			}
			if seriesDepth > 0 && depth == seriesDepth+1 && insertAt < 0 {
				switch t.Name.Local {
				case "dLbls", "trendline", "errBars", "cat", "val", "xVal", "yVal", "smooth", "extLst":
					insertAt = start
				}
			}

		case xml.EndElement:
			if pointStart >= 0 && depth == pointDepth && t.Name.Local == "dPt" {
				if seriesIndex < len(c.Series) {
					if style, ok := c.Series[seriesIndex].PointStyles[pointIndex]; ok {
						edits = append(edits, edit{pointStart, int(d.InputOffset()), chartPointXML(pointIndex, style, c.Series[seriesIndex].PointColors[pointIndex])})
						seen[pointIndex] = true
					}
				}
				pointStart = -1
			}
			if seriesDepth > 0 && depth == seriesDepth && t.Name.Local == "ser" {
				// Excelize normally emits dPt for each PointColors entry. Insert any extra effects before labels/categories.
				if seriesIndex < len(c.Series) {
					var indices []int
					for i := range c.Series[seriesIndex].PointStyles {
						if !seen[i] {
							indices = append(indices, i)
						}
					}
					sort.Ints(indices)
					var extra strings.Builder
					for _, i := range indices {
						extra.WriteString(chartPointXML(i, c.Series[seriesIndex].PointStyles[i], c.Series[seriesIndex].PointColors[i]))
					}
					if extra.Len() > 0 {
						if insertAt < 0 {
							insertAt = start
						}
						edits = append(edits, edit{insertAt, insertAt, extra.String()})
					}
				}
				seriesDepth = 0
			}
			if depth == layoutDepth && t.Name.Local == "layout" && c.PlotArea != nil {
				edits = append(edits, edit{layoutStart, int(d.InputOffset()), chartLayoutXML(c.PlotArea)})
			}
			if depth == plotDepth && t.Name.Local == "plotArea" {
				plotDepth = 0
			}
			depth--
		}
	}
	// Generated charts include a layout element; handle empty plot areas defensively.
	if !layoutSeen && c.PlotArea != nil {
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			t, e := dec.Token()
			if e != nil {
				break
			}
			if el, ok := t.(xml.StartElement); ok && el.Name.Local == "plotArea" {
				at := int(dec.InputOffset())
				edits = append(edits, edit{at, at, chartLayoutXML(c.PlotArea)})
				break
			}
		}
	}
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		data = append(append(append([]byte{}, data[:e.start]...), e.text...), data[e.end:]...)
	}
	return data
}

const chartNamespaces = ` xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`

func chartLayoutXML(p *model.ChartPlotArea) string {
	return fmt.Sprintf(`<c:layout%s><c:manualLayout><c:layoutTarget val="inner"/><c:xMode val="edge"/><c:yMode val="edge"/><c:x val="%g"/><c:y val="%g"/><c:w val="%g"/><c:h val="%g"/></c:manualLayout></c:layout>`, chartNamespaces, p.X, p.Y, p.W, p.H)
}
func chartPointXML(index int, p model.ChartPointStyle, color string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<c:dPt%s><c:idx val="%d"/><c:spPr>`, chartNamespaces, index)
	if g := p.Gradient; g != nil {
		b.WriteString(`<a:gradFill><a:gsLst>`)
		for _, s := range g.Stops {
			fmt.Fprintf(&b, `<a:gs pos="%.0f"><a:srgbClr val="%s"/></a:gs>`, s.Position*100000, chartHex(s.Color))
		}
		fmt.Fprintf(&b, `</a:gsLst><a:lin ang="%.0f" scaled="0"/></a:gradFill>`, g.Angle*60000)
	} else if color != "" {
		fmt.Fprintf(&b, `<a:solidFill><a:srgbClr val="%s"/></a:solidFill>`, chartHex(color))
	}
	if p.LineHidden {
		b.WriteString(`<a:ln><a:noFill/></a:ln>`)
	}
	if s := p.Shadow; s != nil {
		angle := math.Atan2(s.Y, s.X) * 180 / math.Pi
		if angle < 0 {
			angle += 360
		}
		fmt.Fprintf(&b, `<a:effectLst><a:outerShdw blurRad="%.0f" dist="%.0f" dir="%.0f" rotWithShape="0"><a:srgbClr val="%s"><a:alpha val="%.0f"/></a:srgbClr></a:outerShdw></a:effectLst>`, s.Blur*emuPerPx, math.Hypot(s.X, s.Y)*emuPerPx, angle*60000, chartHex(s.Color), s.Opacity*100000)
	}
	b.WriteString(`</c:spPr></c:dPt>`)
	return b.String()
}

func chartHex(color string) string {
	color = normalizeHex(color)
	if _, err := strconv.ParseUint(color, 16, 24); err != nil {
		return "000000"
	}
	return color
}
