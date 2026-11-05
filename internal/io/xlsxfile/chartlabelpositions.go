package xlsxfile

import "strings"

// Office restricts series label positions by chart group beyond the XML schema.
// https://learn.microsoft.com/en-us/openspecs/office_standards/ms-oe376/7dd610ae-d805-40a4-ae3b-c171b6bf08e3
func repairChartLabelPositions(data []byte) []byte {
	root, err := drawingNodes(data)
	if err != nil {
		return data
	}
	var edits []drawingEdit
	for _, position := range root.all("dLblPos") {
		series := false
		for parent := position.parent; parent != nil; parent = parent.parent {
			if parent.name == "ext" || parent.name == "extLst" {
				break
			}
			if parent.name == "ser" {
				series = true
			}
			if !series {
				continue
			}
			allowed := ""
			switch parent.name {
			case "pieChart", "pie3DChart", "ofPieChart":
				allowed = "|bestFit|outEnd|inEnd|ctr|"
			case "barChart":
				allowed = "|inBase|inEnd|ctr|"
				if parent.child("grouping").attr("val") == "clustered" {
					allowed += "outEnd|"
				}
			case "lineChart", "scatterChart", "bubbleChart", "stockChart":
				allowed = "|l|r|b|t|ctr|"
			case "areaChart", "area3DChart", "bar3DChart", "doughnutChart", "line3DChart", "radarChart":
				allowed = "|"
			default:
				continue
			}
			if !strings.Contains(allowed, "|"+position.attr("val")+"|") {
				// Omission selects Excel's automatic position. Manual label layouts
				// and custom text remain untouched.
				edits = append(edits, drawingEdit{position.start, position.end, ""})
			}
			break
		}
	}
	return applyDrawingEdits(data, edits)
}
