package xlsxfile

import (
	"fmt"
	"reflect"
	"strconv"
	"ww-excel/internal/model"
)

var drawingFillOrder = []string{"noFill", "solidFill", "gradFill", "blipFill", "pattFill", "grpFill", "ln", "prstDash", "custDash", "round", "bevel", "miter", "headEnd", "tailEnd", "effectLst", "effectDag", "scene3d", "sp3d", "extLst"}

func chartSeriesFill(raw []byte, color string) string {
	root, _ := drawingNodes(raw)
	replacements := []chartReplacement{}
	for _, kind := range []string{"noFill", "solidFill", "gradFill", "blipFill", "pattFill", "grpFill"} {
		value := ""
		if kind == "solidFill" && color != "" {
			value = `<a:solidFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:srgbClr val="` + chartHex(color) + `"/></a:solidFill>`
		}
		replacements = append(replacements, chartReplacement{kind, value})
	}
	return replaceChartChildren(raw, root.children[0], replacements, drawingFillOrder)
}
func seriesLineXML(raw []byte, width float64, color string, changeWidth, changeColor bool) string {
	root, _ := drawingNodes(raw)
	if changeWidth {
		// Automatic width is the renderer's default 2.25 logical pixels.
		if width == 0 {
			width = 2.25
		}
		raw = applyDrawingEdits(raw, []drawingEdit{drawingAttribute(raw, root.children[0], "w", strconv.FormatFloat(width*emuPerPx, 'f', 0, 64))})
	}
	if changeColor {
		return chartSeriesFill(raw, color)
	}
	return string(raw)
}
func seriesShapeXML(data []byte, node *drawingNode, old, new model.ChartSeries, kind string) string {
	raw := []byte(chartElement("spPr", ""))
	if n := node.child("spPr"); n != nil {
		raw = []byte(standaloneChartNode(data, n))
	}
	isLine := kind == "line" || kind == "scatter"
	if new.Color != old.Color && !isLine {
		raw = []byte(chartSeriesFill(raw, new.Color))
	}
	if isLine && (new.Color != old.Color || new.LineWidth != old.LineWidth) {
		root, _ := drawingNodes(raw)
		sp := root.children[0]
		line := []byte(`<a:ln xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`)
		if n := sp.child("ln"); n != nil {
			line = []byte(standaloneChartNode(raw, n))
		}
		patched := seriesLineXML(line, new.LineWidth, new.Color, new.LineWidth != old.LineWidth, new.Color != old.Color)
		raw = []byte(replaceChartChildren(raw, sp, []chartReplacement{{"ln", patched}}, []string{"xfrm", "prstGeom", "noFill", "solidFill", "gradFill", "ln", "effectLst", "effectDag", "extLst"}))
	}
	return string(raw)
}
func seriesMarkerXML(data []byte, node *drawingNode, old, new model.ChartSeries) string {
	raw := []byte(chartElement("marker", ""))
	if n := node.child("marker"); n != nil {
		raw = []byte(standaloneChartNode(data, n))
	}
	root, _ := drawingNodes(raw)
	marker := root.children[0]
	changes := []chartReplacement{}
	if new.Marker != old.Marker {
		symbol := new.Marker
		if symbol == "" {
			symbol = "auto"
		}
		changes = append(changes, chartReplacement{"symbol", chartVal("symbol", symbol)})
	}
	if new.Color != old.Color && new.Marker != "picture" {
		sp := []byte(chartElement("spPr", ""))
		if n := marker.child("spPr"); n != nil {
			sp = []byte(standaloneChartNode(raw, n))
		}
		sp = []byte(chartSeriesFill(sp, new.Color))
		tree, _ := drawingNodes(sp)
		ln := []byte(`<a:ln xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`)
		if n := tree.children[0].child("ln"); n != nil {
			ln = []byte(standaloneChartNode(sp, n))
		}
		spText := replaceChartChildren(sp, tree.children[0], []chartReplacement{{"ln", chartSeriesFill(ln, new.Color)}}, []string{"noFill", "solidFill", "gradFill", "ln", "effectLst", "extLst"})
		changes = append(changes, chartReplacement{"spPr", spText})
	}
	return replaceChartChildren(raw, marker, changes, []string{"symbol", "size", "spPr", "extLst"})
}
func seriesAppearanceChanges(data []byte, node *drawingNode, old, new model.ChartSeries, kind string) []chartReplacement {
	changes := []chartReplacement{}
	if new.Color != old.Color || new.LineWidth != old.LineWidth {
		changes = append(changes, chartReplacement{"spPr", seriesShapeXML(data, node, old, new, kind)})
	}
	if (kind == "line" || kind == "scatter") && (new.Marker != old.Marker || new.Color != old.Color) {
		changes = append(changes, chartReplacement{"marker", seriesMarkerXML(data, node, old, new)})
	}
	return changes
}
func restoreChartPlotStyle(data []byte, c, old *model.Chart) []byte {
	root, err := drawingNodes(data)
	if err != nil {
		return data
	}
	edits := []drawingEdit{}
	if c.Type == "scatter" && old.Type == "" {
		for _, node := range root.all("scatterChart") {
			raw := replaceChartChildren(data, node, []chartReplacement{{"scatterStyle", chartVal("scatterStyle", "marker")}}, []string{"scatterStyle", "varyColors", "ser", "dLbls", "axId", "extLst"})
			edits = append(edits, drawingEdit{node.start, node.end, raw})
		}
	}
	if !reflect.DeepEqual(c.GapWidth, old.GapWidth) {
		gap := 150.
		if c.GapWidth != nil {
			gap = *c.GapWidth
		}
		for _, kind := range []string{"barChart", "bar3DChart"} {
			for _, node := range root.all(kind) {
				raw := replaceChartChildren(data, node, []chartReplacement{{"gapWidth", chartVal("gapWidth", strconv.FormatFloat(gap, 'g', -1, 64))}}, []string{"barDir", "grouping", "varyColors", "ser", "dLbls", "gapWidth", "overlap", "serLines", "axId", "extLst"})
				edits = append(edits, drawingEdit{node.start, node.end, raw})
			}
		}
	}
	if c.HoleSize != old.HoleSize {
		size := c.HoleSize
		if size == 0 {
			size = 50
		}
		for _, node := range root.all("doughnutChart") {
			raw := replaceChartChildren(data, node, []chartReplacement{{"holeSize", chartVal("holeSize", fmt.Sprint(size))}}, []string{"varyColors", "ser", "dLbls", "firstSliceAng", "holeSize", "extLst"})
			edits = append(edits, drawingEdit{node.start, node.end, raw})
		}
	}
	return applyDrawingEdits(data, edits)
}

// Apply typed styles omitted by the generic Excelize chart writer.
func restoreGeneratedSeriesStyles(data []byte, c *model.Chart) []byte {
	root, err := drawingNodes(data)
	if err != nil {
		return data
	}
	used := map[*drawingNode]bool{}
	edits := []drawingEdit{}
	for _, series := range c.Series {
		node := matchChartSeries(root.all("ser"), data, series, used)
		if node == nil {
			continue
		}
		kind := series.Type
		if kind == "" {
			kind = c.Type
		}
		prior := model.ChartSeries{}
		if kind == "line" && series.LineWidth == 0 {
			prior.LineWidth = -1
		}
		raw := replaceChartChildren(data, node, seriesAppearanceChanges(data, node, prior, series, kind), seriesChildOrder)
		edits = append(edits, drawingEdit{node.start, node.end, raw})
	}
	return restoreChartPlotStyle(applyDrawingEdits(data, edits), c, &model.Chart{})
}
