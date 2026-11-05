package xlsxfile

import (
	"fmt"
	"reflect"
	"strconv"
	"ww-excel/internal/model"
)

type chartAxisBinding struct {
	node *drawingNode
	axis *model.ChartAxis
	key  string
}

// Resolve roles by axis direction and cross IDs, independent of XML order.
func chartAxisBindings(root *drawingNode, c *model.Chart) []chartAxisBinding {
	values := root.all("valAx")
	cats := append(root.all("catAx"), root.all("dateAx")...)
	var value, category, secondary, secondaryCategory *drawingNode
	if c.Type == "scatter" {
		for _, n := range values {
			p := n.child("axPos").attr("val")
			if p == "b" || p == "t" {
				if category == nil {
					category = n
				}
			} else if value == nil {
				value = n
			}
		}
	} else {
		if len(values) > 0 {
			value = values[0]
		}
		if len(cats) > 0 {
			category = cats[0]
		}
		if c.Type == "combo" {
			for _, n := range values {
				if n.child("axPos").attr("val") == "l" {
					value = n
					break
				}
			}
			for _, n := range values {
				if n != value {
					secondary = n
					break
				}
			}
			for _, n := range cats {
				if value != nil && n.child("axId").attr("val") == value.child("crossAx").attr("val") {
					category = n
				}
				if secondary != nil && n.child("axId").attr("val") == secondary.child("crossAx").attr("val") {
					secondaryCategory = n
				}
			}
		}
	}
	return []chartAxisBinding{{value, c.ValueAxis, "valueAxis"}, {category, c.CategoryAxis, "categoryAxis"}, {secondary, c.SecondaryAxis, "secondaryAxis"}, {secondaryCategory, c.SecondaryCategoryAxis, "secondaryCategoryAxis"}}
}

var chartAxisOrder = []string{"axId", "scaling", "delete", "axPos", "majorGridlines", "minorGridlines", "title", "numFmt", "majorTickMark", "minorTickMark", "tickLblPos", "spPr", "txPr", "crossAx", "crosses", "crossesAt", "crossBetween", "majorUnit", "minorUnit", "auto", "lblAlgn", "lblOffset", "tickLblSkip", "tickMarkSkip", "noMultiLvlLbl", "extLst"}

func patchAxisFont(data []byte, node *drawingNode, a, old *model.ChartAxis) string {
	tx := node.child("txPr")
	if tx == nil {
		return labelFont(&model.ChartLabels{FontName: a.FontName, FontSize: a.FontSize})
	}
	raw := []byte(standaloneChartNode(data, tx))
	root, _ := drawingNodes(raw)
	edits := []drawingEdit{}
	for _, n := range root.all("defRPr") {
		part := []byte(n.raw(raw))
		tree, _ := drawingNodes(part)
		font := tree.children[0]
		if a.FontName != old.FontName {
			name := a.FontName
			if name == "" {
				name = "+mn-lt"
			}
			changes := []chartReplacement{}
			for _, tag := range []string{"latin", "ea", "cs"} {
				changes = append(changes, chartReplacement{tag, `<a:` + tag + ` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" typeface="` + escapeDrawing(name) + `"/>`})
			}
			part = []byte(replaceChartChildren(part, font, changes, []string{"ln", "noFill", "solidFill", "gradFill", "effectLst", "highlight", "latin", "ea", "cs", "sym", "hlinkClick", "hlinkMouseOver", "extLst"}))
			tree, _ = drawingNodes(part)
			font = tree.children[0]
		}
		if a.FontSize != old.FontSize {
			size := a.FontSize
			if size == 0 {
				size = 9
			}
			part = applyDrawingEdits(part, []drawingEdit{drawingAttribute(part, font, "sz", strconv.FormatFloat(size*100, 'f', 0, 64))})
		}
		edits = append(edits, drawingEdit{n.start, n.end, string(part)})
	}
	if len(edits) == 0 {
		return labelFont(&model.ChartLabels{FontName: a.FontName, FontSize: a.FontSize})
	}
	return string(applyDrawingEdits(raw, edits))
}

func patchChartAxis(data []byte, node *drawingNode, a, prior *model.ChartAxis) string {
	if a == nil {
		a = &model.ChartAxis{}
	}
	if prior == nil {
		prior = &model.ChartAxis{}
	}
	changes := []chartReplacement{}
	add := func(name, raw string) { changes = append(changes, chartReplacement{name, raw}) }
	if !reflect.DeepEqual(a.Minimum, prior.Minimum) || !reflect.DeepEqual(a.Maximum, prior.Maximum) {
		scale := node.child("scaling")
		raw := []byte(chartElement("scaling", ""))
		if scale != nil {
			raw = []byte(standaloneChartNode(data, scale))
		}
		root, _ := drawingNodes(raw)
		edits := []chartReplacement{}
		for _, v := range []struct {
			name  string
			value *float64
		}{{"min", a.Minimum}, {"max", a.Maximum}} {
			text := ""
			if v.value != nil {
				text = chartVal(v.name, strconv.FormatFloat(*v.value, 'g', -1, 64))
			}
			edits = append(edits, chartReplacement{v.name, text})
		}
		add("scaling", replaceChartChildren(raw, root.children[0], edits, []string{"logBase", "orientation", "max", "min", "extLst"}))
	}
	if a.MajorUnit != prior.MajorUnit && node.name != "catAx" {
		raw := ""
		if a.MajorUnit > 0 {
			raw = chartVal("majorUnit", strconv.FormatFloat(a.MajorUnit, 'g', -1, 64))
		}
		add("majorUnit", raw)
	}
	if a.Hidden != prior.Hidden {
		add("delete", chartBoolXML("delete", a.Hidden))
	}
	if a.GridLines != prior.GridLines {
		raw := ""
		if a.GridLines {
			raw = chartElement("majorGridlines", "")
		}
		add("majorGridlines", raw)
	}
	if a.Format != prior.Format {
		format := a.Format
		if format == "" {
			format = "General"
		}
		add("numFmt", `<c:numFmt`+chartNamespaces+` formatCode="`+escapeDrawing(format)+`" sourceLinked="0"/>`)
	}
	if a.MajorTick != prior.MajorTick {
		tick := a.MajorTick
		if tick == "" {
			tick = "none"
		}
		add("majorTickMark", chartVal("majorTickMark", tick))
	}
	if a.HideLabels != prior.HideLabels {
		value := "nextTo"
		if a.HideLabels {
			value = "none"
		}
		add("tickLblPos", chartVal("tickLblPos", value))
	}
	if a.FontName != prior.FontName || a.FontSize != prior.FontSize {
		add("txPr", patchAxisFont(data, node, a, prior))
	}
	if a.LineColor != prior.LineColor || a.LineHidden != prior.LineHidden {
		raw := []byte(chartElement("spPr", ""))
		if n := node.child("spPr"); n != nil {
			raw = []byte(standaloneChartNode(data, n))
		}
		root, _ := drawingNodes(raw)
		sp := root.children[0]
		line := []byte(`<a:ln xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`)
		if n := sp.child("ln"); n != nil {
			line = []byte(standaloneChartNode(raw, n))
		}
		lineRoot, _ := drawingNodes(line)
		fill := `<a:noFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
		name := "noFill"
		if !a.LineHidden {
			name = "solidFill"
			color := a.LineColor
			if color == "" {
				color = "595959"
			}
			fill = fmt.Sprintf(`<a:solidFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:srgbClr val="%s"/></a:solidFill>`, chartHex(color))
		}
		replacements := []chartReplacement{}
		for _, kind := range []string{"noFill", "solidFill", "gradFill", "pattFill", "blipFill", "grpFill"} {
			text := ""
			if kind == name {
				text = fill
			}
			replacements = append(replacements, chartReplacement{kind, text})
		}
		lineText := replaceChartChildren(line, lineRoot.children[0], replacements, []string{"noFill", "solidFill", "gradFill", "pattFill", "prstDash", "custDash", "round", "bevel", "miter", "headEnd", "tailEnd", "extLst"})
		add("spPr", replaceChartChildren(raw, sp, []chartReplacement{{"ln", lineText}}, []string{"xfrm", "prstGeom", "noFill", "solidFill", "gradFill", "ln", "effectLst", "extLst"}))
	}
	return replaceChartChildren(data, node, changes, chartAxisOrder)
}
func restoreChartAxisSettings(data []byte, c, old *model.Chart) []byte {
	root, err := drawingNodes(data)
	if err != nil {
		return data
	}
	current := chartAxisBindings(root, c)
	previous := chartAxisBindings(root, old)
	edits := []drawingEdit{}
	for i, b := range current {
		if b.node == nil || reflect.DeepEqual(b.axis, previous[i].axis) {
			continue
		}
		edits = append(edits, drawingEdit{b.node.start, b.node.end, patchChartAxis(data, b.node, b.axis, previous[i].axis)})
	}
	return applyDrawingEdits(data, edits)
}

// Excelize's scatter X axis is serialized from a category-axis structure.
// Remove category-only children from value axes, including files we previously
// exported, while retaining their IDs, numeric scales and unknown extensions.
func repairChartValueAxes(data []byte) []byte {
	root, err := drawingNodes(data)
	if err != nil {
		return data
	}
	var edits []drawingEdit
	for _, axis := range root.all("valAx") {
		for _, child := range axis.children {
			switch child.name {
			case "auto", "lblAlgn", "lblOffset", "tickLblSkip", "tickMarkSkip", "noMultiLvlLbl":
				edits = append(edits, drawingEdit{child.start, child.end, ""})
			}
		}
	}
	return applyDrawingEdits(data, edits)
}
