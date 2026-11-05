package xlsxfile

import (
	"fmt"
	"strconv"
	"strings"
)

// A custom caption replaces automatic label content in the renderer. Office
// does not reliably inherit the series' visibility flags for a point label.
func restoreCustomLabelText(data []byte) []byte {
	root, err := drawingNodes(data)
	if err != nil {
		return data
	}
	var edits []drawingEdit
	for _, labels := range root.all("dLbls") {
		if labels.parent == nil || labels.parent.name != "ser" {
			continue
		}
		for _, point := range labels.children {
			if point.name != "dLbl" || point.child("tx") == nil {
				continue
			}
			changes := []chartReplacement{}
			for _, flag := range []string{"showLegendKey", "showVal", "showCatName", "showSerName", "showPercent", "showBubbleSize"} {
				changes = append(changes, chartReplacement{flag, chartBoolXML(flag, false)})
			}
			if rich := point.child("tx").child("rich"); rich != nil && plainCustomLabel(rich) {
				styled := styledCustomLabel(data, rich, point, labels)
				changes = append(changes, chartReplacement{"tx", chartElement("tx", styled)}, chartReplacement{"txPr", customLabelProperties(styled)})
			}
			edits = append(edits, drawingEdit{point.start, point.end, replaceChartChildren(data, point, changes, labelOrder)})
		}
	}
	return applyDrawingEdits(data, edits)
}

// Office drops paragraph properties inside a caption's rich text on save.
// Persist them on the label itself so re-opening keeps the same line advance.
func customLabelProperties(styled string) string {
	data := []byte(styled)
	root, _ := drawingNodes(data)
	rich := root.children[0]
	paragraph := rich.child("p")
	if paragraph == nil {
		return ""
	}
	properties := paragraph.child("pPr")
	font := paragraph.child("r").child("rPr")
	fontXML := ""
	if font != nil {
		fontXML = strings.ReplaceAll(strings.ReplaceAll(font.raw(data), "<a:rPr", "<a:defRPr"), "</a:rPr>", "</a:defRPr>")
	}
	p := replaceChartChildren(data, properties, []chartReplacement{{"defRPr", fontXML}}, []string{"lnSpc", "spcBef", "spcAft", "defRPr", "extLst"})
	return chartElement("txPr", rich.child("bodyPr").raw(data)+`<a:lstStyle/><a:p>`+p+`<a:endParaRPr lang="zh-TW"/></a:p>`)
}

// Preserve imported mixed formatting. Only materialize the plain text emitted
// by this app, which has no run or paragraph properties of its own.
func plainCustomLabel(rich *drawingNode) bool {
	if body := rich.child("bodyPr"); body != nil && len(body.attrs) != 0 {
		return false
	}
	if list := rich.child("lstStyle"); list != nil && len(list.children) != 0 {
		return false
	}
	for _, paragraph := range rich.children {
		if paragraph.name != "p" {
			continue
		}
		for _, run := range paragraph.children {
			if run.name != "r" || run.child("rPr") != nil {
				return false
			}
		}
	}
	return true
}

func styledCustomLabel(data []byte, rich, point, labels *drawingNode) string {
	var fonts []*drawingNode
	for _, parent := range []*drawingNode{point, labels} {
		if properties := parent.child("txPr"); properties != nil {
			fonts = append(fonts, properties.all("defRPr")...)
		}
	}
	attribute := func(name, fallback string) string {
		for _, font := range fonts {
			if value := font.attr(name); value != "" {
				return value
			}
		}
		return fallback
	}
	typeface := func(tag, fallback string) string {
		for _, font := range fonts {
			if value := font.child(tag).attr("typeface"); value != "" {
				return value
			}
		}
		return fallback
	}
	size, _ := strconv.ParseFloat(attribute("sz", "1100"), 64)
	var properties strings.Builder
	fmt.Fprintf(&properties, `<a:rPr sz="%.0f" b="%s" i="%s">`, size, escapeDrawing(attribute("b", "0")), escapeDrawing(attribute("i", "0")))
	for _, font := range fonts {
		if fill := font.child("solidFill"); fill != nil {
			properties.WriteString(standaloneChartNode(data, fill))
			break
		}
	}
	fmt.Fprintf(&properties, `<a:latin typeface="%s"/><a:ea typeface="%s"/></a:rPr>`, escapeDrawing(typeface("latin", "Arial")), escapeDrawing(typeface("ea", "PingFang TC")))
	var text strings.Builder
	fmt.Fprintf(&text, `<c:rich%s><a:bodyPr lIns="0" rIns="0" tIns="0" bIns="0" spcFirstLastPara="0"/><a:lstStyle/>`, chartNamespaces)
	for _, paragraph := range rich.children {
		if paragraph.name != "p" {
			continue
		}
		// The canvas uses a fixed 1.2 × font-size line advance. Percentage
		// spacing in Office also includes font-specific ascent/descent metrics.
		fmt.Fprintf(&text, `<a:p><a:pPr algn="ctr"><a:lnSpc><a:spcPts val="%.0f"/></a:lnSpc><a:spcBef><a:spcPts val="0"/></a:spcBef><a:spcAft><a:spcPts val="0"/></a:spcAft></a:pPr>`, size*1.2)
		for _, run := range paragraph.children {
			text.WriteString(`<a:r>` + properties.String())
			for _, child := range run.children {
				text.WriteString(standaloneChartNode(data, child))
			}
			text.WriteString(`</a:r>`)
		}
		text.WriteString(`</a:p>`)
	}
	text.WriteString(`</c:rich>`)
	return text.String()
}
