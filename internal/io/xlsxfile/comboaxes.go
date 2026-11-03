package xlsxfile

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"

	"ww-excel/internal/model"
)

type chartXMLEdit struct {
	start, end int
	text       string
}

func applyChartEdits(data []byte, edits []chartXMLEdit) []byte {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		data = append(append(append([]byte{}, data[:e.start]...), e.text...), data[e.end:]...)
	}
	return data
}

// Restore axis appearance that Excelize does not expose (line visibility,
// tick marks and secondary category labels). Axis IDs/scales stay generated.
func restoreComboAxes(data []byte, c *model.Chart) []byte {
	d := xml.NewDecoder(bytes.NewReader(data))
	depth, start, axisDepth := 0, 0, 0
	values, categories := 0, 0
	var axis *model.ChartAxis
	var edits []chartXMLEdit
	for {
		at := int(d.InputOffset())
		token, err := d.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "valAx" || t.Name.Local == "catAx" || t.Name.Local == "dateAx" {
				axisDepth, start = depth, at
				if t.Name.Local == "valAx" {
					axis = c.ValueAxis
					if values > 0 {
						axis = c.SecondaryAxis
					}
					values++
				} else {
					axis = c.CategoryAxis
					if categories > 0 {
						axis = c.SecondaryCategoryAxis
					}
					categories++
				}
			}
		case xml.EndElement:
			if axisDepth > 0 && depth == axisDepth {
				end := int(d.InputOffset())
				if axis != nil {
					edits = append(edits, chartXMLEdit{start, end, string(restoreAxisStyle(data[start:end], axis))})
				}
				axisDepth = 0
			}
			depth--
		}
	}
	return applyChartEdits(data, edits)
}

func restoreAxisStyle(data []byte, a *model.ChartAxis) []byte {
	hidden := "0"
	if a.Hidden {
		hidden = "1"
	}
	labels := "nextTo"
	if a.HideLabels {
		labels = "none"
	}
	tick := a.MajorTick
	if tick == "" {
		tick = "none"
	}
	line := `<a:ln><a:noFill/></a:ln>`
	if !a.LineHidden {
		color := a.LineColor
		if color == "" {
			color = "595959"
		}
		line = fmt.Sprintf(`<a:ln><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:ln>`, chartHex(color))
	}
	replacements := map[string]string{
		"delete":        fmt.Sprintf(`<c:delete%s val="%s"/>`, chartNamespaces, hidden),
		"majorTickMark": fmt.Sprintf(`<c:majorTickMark%s val="%s"/>`, chartNamespaces, xmlEscape(tick)),
		"tickLblPos":    fmt.Sprintf(`<c:tickLblPos%s val="%s"/>`, chartNamespaces, labels),
		"spPr":          fmt.Sprintf(`<c:spPr%s><a:noFill/>%s</c:spPr>`, chartNamespaces, line),
	}
	if a.FontSize > 0 || a.FontName != "" {
		replacements["txPr"] = fmt.Sprintf(`<c:txPr%s><a:bodyPr/><a:lstStyle/><a:p><a:pPr><a:defRPr sz="%.0f"><a:latin typeface="%s"/></a:defRPr></a:pPr></a:p></c:txPr>`, chartNamespaces, axisFontSize(a.FontSize)*100, xmlEscape(a.FontName))
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	depth, childStart, childDepth, insertAt := 0, 0, 0, -1
	key := ""
	seen := map[string]bool{}
	var edits []chartXMLEdit
	for {
		at := int(d.InputOffset())
		token, err := d.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				if _, ok := replacements[t.Name.Local]; ok {
					childStart, childDepth, key = at, depth, t.Name.Local
					seen[key] = true
				}
				if t.Name.Local == "crossAx" {
					insertAt = at
				}
			}
		case xml.EndElement:
			if childDepth > 0 && depth == childDepth {
				edits = append(edits, chartXMLEdit{childStart, int(d.InputOffset()), replacements[key]})
				childDepth = 0
			}
			if depth == 1 && insertAt < 0 {
				insertAt = at
			}
			depth--
		}
	}
	var missing bytes.Buffer
	for _, key := range []string{"delete", "majorTickMark", "tickLblPos", "spPr", "txPr"} {
		if value, ok := replacements[key]; ok && !seen[key] {
			missing.WriteString(value)
		}
	}
	if missing.Len() > 0 && insertAt >= 0 {
		edits = append(edits, chartXMLEdit{insertAt, insertAt, missing.String()})
	}
	return applyChartEdits(data, edits)
}

func axisFontSize(size float64) float64 {
	if size <= 0 {
		return 11
	}
	return size
}
