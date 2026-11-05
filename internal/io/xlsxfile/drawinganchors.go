package xlsxfile

import (
	"fmt"
	"strconv"
	"strings"
	"ww-excel/internal/model"
)

// Excelize can emit a two-cell anchor with oneCell positioning whose end
// marker remains in the start cell and stores the entire size as an offset.
// Excel clamps those offsets. Use an explicit extent for one-cell positioning;
// resolve markers into their actual cells for two-cell positioning.
func normalizeChartDrawingAnchors(data []byte, layout *model.Layout) []byte {
	root, err := drawingNodes(data)
	if err != nil {
		return data
	}
	var edits []drawingEdit
	readMarker := func(marker *drawingNode) model.Anchor {
		value := func(name string) int64 {
			node := marker.child(name)
			if node == nil {
				return 0
			}
			v, _ := strconv.ParseInt(string(data[node.open:node.close]), 10, 64)
			return v
		}
		return model.Anchor{Col: int(value("col")), Row: int(value("row")), DX: float64(value("colOff")) / emuPerPx, DY: float64(value("rowOff")) / emuPerPx}
	}
	for _, anchor := range root.all("twoCellAnchor") {
		if anchor.child("graphicFrame") == nil {
			continue
		}
		if anchor.attr("editAs") == "oneCell" && anchor.child("from") != nil && anchor.child("to") != nil {
			from, to := readMarker(anchor.child("from")), readMarker(anchor.child("to"))
			width, height := anchorSize(layout, rawAnchor{twoCell: true, FromCol: from.Col, FromRow: from.Row, FromCX: int64(mathRound(from.DX * emuPerPx)), FromRY: int64(mathRound(from.DY * emuPerPx)), ToCol: to.Col, ToRow: to.Row, ToCX: int64(mathRound(to.DX * emuPerPx)), ToRY: int64(mathRound(to.DY * emuPerPx))})
			// An explicit extent avoids Excel deriving a different size from its
			// platform-specific default column metrics.
			var raw strings.Builder
			fmt.Fprintf(&raw, `<xdr:oneCellAnchor xmlns:xdr="%s">`, spreadsheetDrawingNS)
			raw.WriteString(markerXML("from", shiftAnchor(layout, from, 0, 0)))
			fmt.Fprintf(&raw, `<xdr:ext cx="%d" cy="%d"/>`, int64(mathRound(width*emuPerPx)), int64(mathRound(height*emuPerPx)))
			for _, child := range anchor.children {
				if child.name != "from" && child.name != "to" {
					raw.WriteString(child.raw(data))
				}
			}
			raw.WriteString(`</xdr:oneCellAnchor>`)
			edits = append(edits, drawingEdit{anchor.start, anchor.end, raw.String()})
			continue
		}
		for _, name := range []string{"from", "to"} {
			marker := anchor.child(name)
			if marker == nil {
				continue
			}
			a := readMarker(marker)
			normalized := shiftAnchor(layout, a, 0, 0)
			if normalized != a {
				edits = append(edits, drawingEdit{marker.start, marker.end, markerXML(name, normalized)})
			}
		}
	}
	return applyDrawingEdits(data, edits)
}

func hasChartObjects(objects []model.Object) bool {
	for _, object := range objects {
		if object.Chart != nil {
			return true
		}
	}
	return false
}
