package xlsxfile

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"

	"ww-excel/internal/model"
)

// Excelize accepts category references, so write literal categories separately.
// Empty points remain empty instead of becoming generated numeric labels.
func restoreChartCategories(data []byte, series []model.ChartSeries) []byte {
	type edit struct {
		start, end int
		text       []byte
	}
	var edits []edit
	d := xml.NewDecoder(bytes.NewReader(data))
	depth, seriesDepth, index, catStart, catEnd, insertAt := 0, 0, -1, -1, -1, -1
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
				catStart, catEnd, insertAt = -1, -1, -1
			}
			if seriesDepth > 0 && depth == seriesDepth+1 {
				if t.Name.Local == "cat" {
					catStart = start
				}
				if insertAt < 0 && (t.Name.Local == "val" || t.Name.Local == "extLst") {
					insertAt = start
				}
			}
		case xml.EndElement:
			if catStart >= 0 && depth == seriesDepth+1 && t.Name.Local == "cat" {
				catEnd = int(d.InputOffset())
			}
			if depth == seriesDepth && t.Name.Local == "ser" {
				if index < len(series) && series[index].Categories == "" && series[index].CategoryLabels != nil {
					labels := series[index].CategoryLabels
					var raw bytes.Buffer
					fmt.Fprintf(&raw, `<c:cat xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:strLit><c:ptCount val="%d"/>`, len(labels))
					for i, label := range labels {
						fmt.Fprintf(&raw, `<c:pt idx="%d"><c:v>`, i)
						xml.EscapeText(&raw, []byte(label))
						raw.WriteString(`</c:v></c:pt>`)
					}
					raw.WriteString(`</c:strLit></c:cat>`)
					if catEnd < 0 {
						if insertAt < 0 {
							insertAt = start
						}
						catStart, catEnd = insertAt, insertAt
					}
					edits = append(edits, edit{catStart, catEnd, raw.Bytes()})
				}
				seriesDepth = 0
			}
			depth--
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		data = append(append(append([]byte{}, data[:e.start]...), e.text...), data[e.end:]...)
	}
	return data
}
