package xlsxfile

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"

	"ww-excel/internal/model"
)

// Excelize creates x14 bars with automatic bounds even when the legacy rule
// has fixed thresholds. Match each extension by its legacy UUID, preserving
// unrelated extension properties and rules byte for byte.
func replaceWorksheetBarBounds(data []byte, rules []model.CFRule) ([]byte, error) {
	byRange := map[string][]*model.CFDataBar{}
	for _, rule := range rules {
		if rule.Type == "data_bar" {
			key := formatSqref(rule.Ranges)
			byRange[key] = append(byRange[key], rule.DataBar)
		}
	}
	if len(byRange) == 0 {
		return data, nil
	}
	byID := map[string]*model.CFDataBar{}
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		el, ok := token.(xml.StartElement)
		if !ok || el.Name.Local != "conditionalFormatting" || el.Name.Space != "http://schemas.openxmlformats.org/spreadsheetml/2006/main" {
			continue
		}
		var group rawConditionalGroup
		if err := d.DecodeElement(&group, &el); err != nil {
			return nil, err
		}
		key := formatSqref(parseSqref(group.Range))
		for _, rule := range group.Rules {
			if rule.Bar == nil {
				continue
			}
			candidates := byRange[key]
			if len(candidates) == 0 {
				continue
			}
			bar := candidates[0]
			byRange[key] = candidates[1:]
			if bar != nil && len(rule.IDs) > 0 {
				byID[rule.IDs[0]] = bar
			}
		}
	}
	if len(byID) == 0 {
		return data, nil
	}
	d = xml.NewDecoder(bytes.NewReader(data))
	depth, ruleDepth, barDepth := 0, 0, 0
	var active *model.CFDataBar
	var edits []cfXMLEdit
	thresholdIndex := 0
	for {
		start := int(d.InputOffset())
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch el := token.(type) {
		case xml.StartElement:
			depth++
			if el.Name.Space == "http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" && el.Name.Local == "cfRule" {
				active = byID[attr(el, "id")]
				ruleDepth = depth
			}
			if active != nil && depth == ruleDepth+1 && el.Name.Local == "dataBar" {
				barDepth = depth
				thresholdIndex = 0
			}
			if active != nil && barDepth > 0 && depth == barDepth+1 && el.Name.Local == "cfvo" {
				var original rawCFThreshold
				if err := d.DecodeElement(&original, &el); err != nil {
					return nil, err
				}
				if thresholdIndex < 2 {
					threshold := active.Minimum
					fallback := "min"
					if thresholdIndex == 1 {
						threshold = active.Maximum
						fallback = "max"
					}
					if threshold.Type == "" {
						threshold.Type = fallback
					}
					edits = append(edits, cfXMLEdit{start, int(d.InputOffset()), barThresholdXML(threshold)})
				}
				thresholdIndex++
				depth--
			}
		case xml.EndElement:
			if depth == barDepth {
				barDepth = 0
			}
			if depth == ruleDepth {
				ruleDepth = 0
				active = nil
			}
			depth--
		}
	}
	return applyCFEdits(data, edits), nil
}

func barThresholdXML(threshold model.CFThreshold) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, `<x14:cfvo xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" type="%s" gte="%t">`, escapeCF(threshold.Type), !threshold.Strict)
	if threshold.Value != "" {
		fmt.Fprintf(&b, `<xm:f xmlns:xm="http://schemas.microsoft.com/office/excel/2006/main">%s</xm:f>`, escapeCF(threshold.Value))
	}
	b.WriteString(`</x14:cfvo>`)
	return b.Bytes()
}
