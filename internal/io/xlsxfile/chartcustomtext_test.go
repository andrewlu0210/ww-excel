package xlsxfile

import (
	"strings"
	"testing"
)

func TestCustomCaptionDisablesAutomaticContentAndMaterializesTextStyle(t *testing.T) {
	raw := []byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><c:chart><c:plotArea><c:lineChart><c:ser><c:dLbls><c:dLbl><c:idx val="1"/><c:layout><c:manualLayout><c:x val="0.5"/><c:y val="0.3"/></c:manualLayout></c:layout><c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>自訂收入</a:t></a:r></a:p><a:p><a:r><a:t>150 元</a:t></a:r></a:p></c:rich></c:tx><c:extLst><c:ext uri="keep"/></c:extLst></c:dLbl><c:txPr><a:bodyPr/><a:p><a:pPr><a:defRPr sz="1600" b="1"/></a:pPr></a:p></c:txPr><c:showVal val="1"/></c:dLbls></c:ser></c:lineChart></c:plotArea></c:chart></c:chartSpace>`)
	fixed := restoreCustomLabelText(raw)
	root, err := drawingNodes(fixed)
	if err != nil {
		t.Fatal(err)
	}
	point := root.all("dLbl")[0]
	for _, flag := range []string{"showLegendKey", "showVal", "showCatName", "showSerName", "showPercent", "showBubbleSize"} {
		if point.child(flag).attr("val") != "0" {
			t.Fatalf("automatic content active: %s", flag)
		}
	}
	for _, token := range []string{`typeface="PingFang TC"`, `algn="ctr"`, `val="1920"`, `sz="1600"`, `b="1"`, `uri="keep"`, `自訂收入`, `150 元`, `val="0.5"`} {
		if !strings.Contains(string(fixed), token) {
			t.Fatalf("missing %s", token)
		}
	}
	properties := point.child("txPr")
	if properties == nil || properties.child("p").child("pPr").child("lnSpc").child("spcPts").attr("val") != "1920" {
		t.Fatal("must persist line spacing on the label, not only its caption")
	}
	if string(restoreCustomLabelText(fixed)) != string(fixed) {
		t.Fatal("must be idempotent")
	}
}
func TestImportedRichCaptionFormattingIsPreserved(t *testing.T) {
	raw := []byte(`<c:dLbls xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:dLbl><c:tx><c:rich xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:bodyPr rot="120000"/><a:p><a:pPr algn="r"/><a:r><a:rPr sz="2000"/><a:t>原有文字</a:t></a:r></a:p></c:rich></c:tx></c:dLbl></c:dLbls>`)
	root, err := drawingNodes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if plainCustomLabel(root.all("rich")[0]) {
		t.Fatal("must preserve explicit rich formatting")
	}
	if string(restoreCustomLabelText(raw)) != string(raw) {
		t.Fatal("non-series labels must remain unchanged")
	}
	series := []byte(`<c:ser xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">` + string(raw) + `</c:ser>`)
	fixed := restoreCustomLabelText(series)
	after, err := drawingNodes(fixed)
	if err != nil {
		t.Fatal(err)
	}
	if after.all("rich")[0].raw(fixed) != root.all("rich")[0].raw(raw) {
		t.Fatal("imported mixed formatting must be retained inside a series")
	}
}
