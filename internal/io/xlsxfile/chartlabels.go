package xlsxfile

import (
	"bytes"
	"encoding/xml"
	"io"
	"sort"

	"ww-excel/internal/model"
)

// Retain label settings that Excelize cannot express, including per-point
// visibility and manual layout. Offsets come from XML tokens, not text matching.
func restoreChartLabels(data []byte, series []model.ChartSeries) []byte {
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	d := xml.NewDecoder(bytes.NewReader(data))
	depth, seriesDepth, index, labelStart, labelEnd, labelDepth, insertAt := 0, 0, -1, -1, -1, 0, -1
	for {
		start := int(d.InputOffset())
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return data
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "ser" {
				index++
				seriesDepth = depth
				labelStart, labelEnd, insertAt = -1, -1, -1
			}
			if seriesDepth > 0 && depth == seriesDepth+1 && t.Name.Local == "dLbls" {
				labelStart, labelDepth = start, depth
			}
			if seriesDepth > 0 && depth == seriesDepth+1 && insertAt < 0 {
				switch t.Name.Local {
				case "trendline", "errBars", "cat", "val", "xVal", "yVal", "smooth", "extLst":
					insertAt = start
				}
			}
		case xml.EndElement:
			preserve := index >= 0 && index < len(series) && series[index].Labels != nil
			if labelStart >= 0 && depth == labelDepth && t.Name.Local == "dLbls" {
				labelEnd = int(d.InputOffset())
			}
			if depth == seriesDepth && t.Name.Local == "ser" {
				if preserve {
					raw := chartLabelsXML(series[index].Labels)
					if labelEnd >= 0 {
						edits = append(edits, edit{labelStart, labelEnd, raw})
					} else {
						if insertAt < 0 {
							insertAt = start
						}
						edits = append(edits, edit{insertAt, insertAt, raw})
					}
				}
				seriesDepth = 0
			}
			depth--
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		data = append(append(append([]byte{}, data[:e.start]...), []byte(e.text)...), data[e.end:]...)
	}
	return data
}
