package xlsxfile

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"ww-excel/internal/model"
)

func chartBool(s string) bool { return s == "1" || s == "true" || s == "on" }
func labelConfig(l *model.ChartLabels) string {
	if l == nil {
		return ""
	}
	copy := *l
	copy.RawXML = ""
	copy.SourceConfig = ""
	b, _ := json.Marshal(copy)
	return string(b)
}
func chartElement(name, inner string) string {
	return `<c:` + name + chartNamespaces + `>` + inner + `</c:` + name + `>`
}
func chartVal(name, value string) string {
	return `<c:` + name + chartNamespaces + ` val="` + escapeDrawing(value) + `"/>`
}
func chartBoolXML(name string, value bool) string {
	return chartVal(name, strconv.Itoa(boolInt(value)))
}

type chartReplacement struct{ Name, XML string }

// Replace direct children without reserializing untouched subtrees or extensions.
func replaceChartChildren(data []byte, parent *drawingNode, changes []chartReplacement, order []string) string {
	type item struct{ name, raw string }
	items := []item{}
	for _, child := range parent.children {
		items = append(items, item{child.name, child.raw(data)})
	}
	rank := func(name string) int {
		for i, n := range order {
			if n == name {
				return i
			}
		}
		return len(order) - 1
	}
	for _, change := range changes {
		next := []item{}
		found := false
		for _, old := range items {
			if old.name == change.Name {
				if !found && change.XML != "" {
					next = append(next, item{change.Name, change.XML})
				}
				found = true
			} else {
				next = append(next, old)
			}
		}
		if !found && change.XML != "" {
			at := len(next)
			for i, node := range next {
				if rank(node.name) > rank(change.Name) {
					at = i
					break
				}
			}
			next = append(next, item{})
			copy(next[at+1:], next[at:])
			next[at] = item{change.Name, change.XML}
		}
		items = next
	}
	head, tail := string(data[parent.start:parent.open]), string(data[parent.close:parent.end])
	if strings.HasSuffix(head, "/>") {
		head = strings.TrimSuffix(head, "/>") + ">"
		name := strings.FieldsFunc(head[1:], func(r rune) bool { return r == ' ' || r == '>' })[0]
		tail = "</" + name + ">"
	}
	var content strings.Builder
	content.WriteString(head)
	for _, node := range items {
		content.WriteString(node.raw)
	}
	content.WriteString(tail)
	return content.String()
}

var labelOrder = []string{"idx", "dLbl", "delete", "layout", "tx", "numFmt", "spPr", "txPr", "dLblPos", "showLegendKey", "showVal", "showCatName", "showSerName", "showPercent", "showBubbleSize", "separator", "showLeaderLines", "leaderLines", "extLst"}

func labelBox(fill, border string) string {
	if fill == "" && border == "" {
		return ""
	}
	var b strings.Builder
	if fill != "" {
		b.WriteString(`<a:solidFill><a:srgbClr val="` + chartHex(fill) + `"/></a:solidFill>`)
	} else {
		b.WriteString(`<a:noFill/>`)
	}
	if border != "" {
		b.WriteString(`<a:ln><a:solidFill><a:srgbClr val="` + chartHex(border) + `"/></a:solidFill></a:ln>`)
	} else {
		b.WriteString(`<a:ln><a:noFill/></a:ln>`)
	}
	return chartElement("spPr", b.String())
}
func labelFont(l *model.ChartLabels) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<c:txPr%s><a:bodyPr rot="%g"/><a:lstStyle/><a:p><a:pPr><a:defRPr b="%d" i="%d"`, chartNamespaces, l.Rotation*60000, boolInt(l.Bold), boolInt(l.Italic))
	if l.FontSize > 0 {
		fmt.Fprintf(&b, ` sz="%g"`, l.FontSize*100)
	}
	b.WriteString(`>`)
	if l.Color != "" {
		b.WriteString(`<a:solidFill><a:srgbClr val="` + chartHex(l.Color) + `"/></a:solidFill>`)
	}
	if l.FontName != "" {
		name := escapeDrawing(l.FontName)
		b.WriteString(`<a:latin typeface="` + name + `"/><a:ea typeface="` + name + `"/>`)
	}
	b.WriteString(`</a:defRPr></a:pPr><a:endParaRPr lang="zh-TW"/></a:p></c:txPr>`)
	return b.String()
}
func labelTextXML(text string) string {
	var b strings.Builder
	b.WriteString(`<c:tx` + chartNamespaces + `><c:rich><a:bodyPr/><a:lstStyle/>`)
	for _, line := range strings.Split(text, "\n") {
		b.WriteString(`<a:p><a:r><a:t xml:space="preserve">` + escapeDrawing(line) + `</a:t></a:r></a:p>`)
	}
	b.WriteString(`</c:rich></c:tx>`)
	return b.String()
}
func labelLayoutXML(l *model.ChartLabelLayout) string {
	if l == nil {
		return ""
	}
	xmode, ymode := l.XMode, l.YMode
	if xmode == "" {
		xmode = "edge"
	}
	if ymode == "" {
		ymode = "edge"
	}
	inner := chartVal("xMode", xmode) + chartVal("yMode", ymode) + chartVal("x", fmt.Sprint(l.X)) + chartVal("y", fmt.Sprint(l.Y))
	if l.W > 0 {
		inner += chartVal("w", fmt.Sprint(l.W))
	}
	if l.H > 0 {
		inner += chartVal("h", fmt.Sprint(l.H))
	}
	return chartElement("layout", chartElement("manualLayout", inner))
}
func pointLabelXML(point model.ChartLabelPoint, base model.ChartLabelPoint, index int, old string) string {
	if old == "" {
		old = chartElement("dLbl", chartVal("idx", strconv.Itoa(index)))
	}
	data := []byte(old)
	root, err := drawingNodes(data)
	if err != nil || len(root.children) != 1 {
		return old
	}
	changes := []chartReplacement{}
	add := func(name, value string) { changes = append(changes, chartReplacement{name, value}) }
	if point.Hidden != base.Hidden {
		add("delete", chartBoolXML("delete", point.Hidden))
	}
	if point.Position != base.Position {
		value := ""
		if point.Position != "" {
			value = chartVal("dLblPos", point.Position)
		}
		add("dLblPos", value)
	}
	if point.Format != base.Format {
		value := ""
		if point.Format != "" {
			value = `<c:numFmt` + chartNamespaces + ` formatCode="` + escapeDrawing(point.Format) + `" sourceLinked="0"/>`
		}
		add("numFmt", value)
	}
	if !reflect.DeepEqual(point.CustomText, base.CustomText) || point.Reference != base.Reference {
		text := ""
		if point.Reference != "" {
			text = chartElement("tx", chartElement("strRef", chartElement("f", escapeDrawing(point.Reference))))
		} else if point.CustomText != nil {
			text = labelTextXML(*point.CustomText)
		}
		add("tx", text)
	}
	for _, flag := range []struct {
		name        string
		value, base *bool
	}{{"showVal", point.ShowValue, base.ShowValue}, {"showPercent", point.ShowPercent, base.ShowPercent}, {"showCatName", point.ShowCategory, base.ShowCategory}, {"showSerName", point.ShowSeries, base.ShowSeries}} {
		if !reflect.DeepEqual(flag.value, flag.base) {
			xml := ""
			if flag.value != nil {
				xml = chartBoolXML(flag.name, *flag.value)
			}
			add(flag.name, xml)
		}
	}
	if point.FontName != base.FontName || point.FontSize != base.FontSize || point.Color != base.Color || !reflect.DeepEqual(point.Bold, base.Bold) || !reflect.DeepEqual(point.Italic, base.Italic) {
		font := &model.ChartLabels{FontName: point.FontName, FontSize: point.FontSize, Color: point.Color}
		if point.Bold != nil {
			font.Bold = *point.Bold
		}
		if point.Italic != nil {
			font.Italic = *point.Italic
		}
		fontXML := labelFont(font)
		if point.Bold == nil {
			fontXML = strings.Replace(fontXML, ` b="0"`, "", 1)
		}
		if point.Italic == nil {
			fontXML = strings.Replace(fontXML, ` i="0"`, "", 1)
		}
		add("txPr", fontXML)
	}
	if point.Center != base.Center {
		if point.Center {
			add("layout", labelLayoutXML(&model.ChartLabelLayout{W: 1, H: 1, XMode: "edge", YMode: "edge"}))
		} else {
			add("layout", labelLayoutXML(point.Layout))
			if ext := root.children[0].child("extLst"); ext != nil {
				xml := ext.raw(data)
				tree, _ := drawingNodes([]byte(xml))
				patches := []drawingEdit{}
				for _, layout := range tree.all("layout") {
					if layout.parent != nil && layout.parent.name == "ext" {
						patches = append(patches, drawingEdit{layout.start, layout.end, ""})
					}
				}
				add("extLst", string(applyDrawingEdits([]byte(xml), patches)))
			}
		}
	}
	if !reflect.DeepEqual(point.Layout, base.Layout) {
		add("layout", labelLayoutXML(point.Layout))
	}
	return replaceChartChildren(data, root.children[0], changes, labelOrder)
}

func chartLabelsXML(l *model.ChartLabels) string {
	if l == nil {
		return ""
	}
	raw := l.RawXML
	if root, e := drawingNodes([]byte(raw)); e != nil || len(root.children) != 1 || root.children[0].name != "dLbls" {
		raw = `<c:dLbls` + chartNamespaces + ` xmlns:c15="http://schemas.microsoft.com/office/drawing/2012/chart">` + raw + `</c:dLbls>`
	}
	if l.RawXML != "" && (l.SourceConfig == "" || labelConfig(l) == l.SourceConfig) {
		return raw
	}
	var base model.ChartLabels
	if l.SourceConfig != "" {
		json.Unmarshal([]byte(l.SourceConfig), &base)
	}
	data := []byte(raw)
	root, err := drawingNodes(data)
	if err != nil || len(root.children) != 1 {
		data = []byte(chartElement("dLbls", ""))
		root, _ = drawingNodes(data)
	}
	node := root.children[0]
	changes := []chartReplacement{}
	fresh := l.RawXML == ""
	add := func(name, value string) { changes = append(changes, chartReplacement{name, value}) }
	if fresh || l.Disabled != base.Disabled {
		add("delete", chartBoolXML("delete", l.Disabled))
	}
	if fresh || l.Format != base.Format || l.PercentFormat != base.PercentFormat || l.ShowValue != base.ShowValue || l.ShowPercent != base.ShowPercent {
		format := l.Format
		if l.ShowPercent && !l.ShowValue {
			format = l.PercentFormat
			if format == "" {
				format = "0%"
			}
		}
		if format == "" {
			format = "General"
		}
		add("numFmt", `<c:numFmt`+chartNamespaces+` formatCode="`+escapeDrawing(format)+`" sourceLinked="0"/>`)
	}
	for _, flag := range []struct {
		name        string
		value, base bool
	}{{"showVal", l.ShowValue, base.ShowValue}, {"showPercent", l.ShowPercent, base.ShowPercent}, {"showCatName", l.ShowCategory, base.ShowCategory}, {"showSerName", l.ShowSeries, base.ShowSeries}, {"showLeaderLines", l.LeaderLines, base.LeaderLines}} {
		if fresh || flag.value != flag.base {
			add(flag.name, chartBoolXML(flag.name, flag.value))
		}
	}
	if fresh || l.Position != base.Position {
		value := ""
		if l.Position != "" {
			value = chartVal("dLblPos", l.Position)
		}
		add("dLblPos", value)
	}
	if fresh || l.Separator != base.Separator {
		sep := l.Separator
		if sep == "" {
			sep = "\n"
		}
		add("separator", chartElement("separator", escapeDrawing(sep)))
	}
	if fresh || l.Fill != base.Fill || l.Border != base.Border {
		add("spPr", labelBox(l.Fill, l.Border))
	}
	if fresh || l.FontName != base.FontName || l.FontSize != base.FontSize || l.Color != base.Color || l.Bold != base.Bold || l.Italic != base.Italic || l.Rotation != base.Rotation {
		add("txPr", labelFont(l))
	}
	raw = replaceChartChildren(data, node, changes, labelOrder)
	data = []byte(raw)
	root, _ = drawingNodes(data)
	node = root.children[0]
	edits := []drawingEdit{}
	seen := map[int]bool{}
	for _, pnode := range node.children {
		if pnode.name != "dLbl" {
			continue
		}
		idx, _ := strconv.Atoi(pnode.child("idx").attr("val"))
		point, exists := l.Points[idx]
		if !exists {
			if _, old := base.Points[idx]; old {
				edits = append(edits, drawingEdit{pnode.start, pnode.end, ""})
			}
			continue
		}
		seen[idx] = true
		if !reflect.DeepEqual(point, base.Points[idx]) {
			edits = append(edits, drawingEdit{pnode.start, pnode.end, pointLabelXML(point, base.Points[idx], idx, pnode.raw(data))})
		}
	}
	indices := []int{}
	for i := range l.Points {
		if !seen[i] {
			indices = append(indices, i)
		}
	}
	sort.Ints(indices)
	var extra strings.Builder
	for _, i := range indices {
		extra.WriteString(pointLabelXML(l.Points[i], model.ChartLabelPoint{}, i, ""))
	}
	if extra.Len() > 0 {
		edits = append(edits, drawingEdit{node.open, node.open, extra.String()})
	}
	return string(applyDrawingEdits(data, edits))
}
