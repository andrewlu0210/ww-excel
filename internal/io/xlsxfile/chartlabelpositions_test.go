package xlsxfile

import (
	"strings"
	"testing"
)

func TestOfficeLabelPositionsByChartGroup(t *testing.T) {
	groups := map[string]string{"barChart": "inBase inEnd outEnd ctr", "lineChart": "l r b t ctr", "scatterChart": "l r b t ctr", "pieChart": "bestFit outEnd inEnd ctr", "doughnutChart": "", "areaChart": "", "radarChart": "", "bar3DChart": ""}
	for group, valid := range groups {
		for _, position := range strings.Fields("inBase inEnd outEnd ctr bestFit l r b t") {
			t.Run(group+"/"+position, func(t *testing.T) {
				raw := []byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea><c:` + group + `><c:grouping val="clustered"/><c:ser><c:dLbls><c:dLbl><c:idx val="1"/><c:layout><c:manualLayout/></c:layout><c:dLblPos val="` + position + `"/></c:dLbl><c:dLblPos val="` + position + `"/></c:dLbls></c:ser><c:dLbls><c:dLblPos val="` + position + `"/></c:dLbls></c:` + group + `></c:plotArea></c:chart></c:chartSpace>`)
				fixed := repairChartLabelPositions(raw)
				expected := 1
				if strings.Contains(" "+valid+" ", " "+position+" ") {
					expected = 3
				}
				if n := strings.Count(string(fixed), "<c:dLblPos"); n != expected {
					t.Fatalf("positions: got %d want %d: %s", n, expected, fixed)
				}
				if !strings.Contains(string(fixed), "<c:manualLayout/>") {
					t.Fatal("lost manual label layout")
				}
				if string(repairChartLabelPositions(fixed)) != string(fixed) {
					t.Fatal("not idempotent")
				}
			})
		}
	}
}
func TestStackedBarsRejectOutsideEndLabelPosition(t *testing.T) {
	raw := []byte(`<barChart><grouping val="stacked"/><ser><dLbls><dLblPos val="outEnd"/></dLbls></ser></barChart>`)
	if strings.Contains(string(repairChartLabelPositions(raw)), "dLblPos") {
		t.Fatal("stacked bar cannot use outside end")
	}
}
