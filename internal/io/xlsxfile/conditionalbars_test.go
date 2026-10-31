package xlsxfile

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"ww-excel/internal/model"
)

func TestDataBarExtensionBoundsMatchLegacyIDs(t *testing.T) {
	legacy := `<conditionalFormatting sqref="A1:A2"><cfRule type="dataBar"><dataBar><cfvo type="num" val="0"/><cfvo type="num" val="100"/></dataBar><extLst><ext><x14:id>first</x14:id></ext></extLst></cfRule><cfRule type="dataBar"><dataBar><cfvo type="min"/><cfvo type="max"/></dataBar><extLst><ext><x14:id>second</x14:id></ext></extLst></cfRule></conditionalFormatting>`
	extension := func(id string) string {
		return `<x14:conditionalFormatting><x14:cfRule type="dataBar" id="` + id + `"><x14:dataBar border="1" gradient="0"><x14:cfvo type="autoMin"/><x14:cfvo type="autoMax"/><x14:borderColor rgb="FF638EC6"/></x14:dataBar></x14:cfRule><xm:sqref>A1:A2</xm:sqref></x14:conditionalFormatting>`
	}
	unrelated := `<other:feature xmlns:other="urn:other" key="keep">unchanged</other:feature>`
	data := []byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" xmlns:xm="http://schemas.microsoft.com/office/excel/2006/main">` + legacy + `<extLst><ext><x14:conditionalFormattings>` + extension("second") + extension("first") + extension("unmatched") + `</x14:conditionalFormattings>` + unrelated + `</ext></extLst></worksheet>`)
	rules := []model.CFRule{
		{Type: "data_bar", Ranges: []model.Range{{R1: 1}}, DataBar: &model.CFDataBar{Minimum: model.CFThreshold{Type: "num", Value: "0"}, Maximum: model.CFThreshold{Type: "num", Value: "100"}}},
		{Type: "data_bar", Ranges: []model.Range{{R1: 1}}, DataBar: &model.CFDataBar{Minimum: model.CFThreshold{Type: "formula", Value: "'資料 & 設定'!$B$1", Strict: true}, Maximum: model.CFThreshold{Type: "percentile", Value: "80"}}},
	}
	got, err := replaceWorksheetBarBounds(data, rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, unchanged := range []string{legacy, unrelated, extension("unmatched"), `border="1" gradient="0"`, `<x14:borderColor rgb="FF638EC6"/>`} {
		if !strings.Contains(string(got), unchanged) {
			t.Fatalf("unrelated XML changed: %s", unchanged)
		}
	}
	var doc struct {
		Groups []rawConditionalGroup `xml:"extLst>ext>conditionalFormattings>conditionalFormatting"`
	}
	if err := xml.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	for _, group := range doc.Groups {
		for _, r := range group.Rules {
			id := ""
			for _, a := range r.Attrs {
				if a.Name.Local == "id" {
					id = a.Value
				}
			}
			index := 0
			if id == "second" {
				index = 1
			}
			if id == "unmatched" {
				continue
			}
			if r.Bar == nil || len(r.Bar.Thresholds) != 2 || r.Bar.Thresholds[0].model() != rules[index].DataBar.Minimum || r.Bar.Thresholds[1].model() != rules[index].DataBar.Maximum {
				t.Fatalf("wrong thresholds for %s: %+v", id, r.Bar)
			}
		}
	}
	twice, err := replaceWorksheetBarBounds(got, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, twice) {
		t.Fatal("repeated save changes patched thresholds")
	}
}

func TestDataBarWithoutSettingsKeepsFollowingRuleAligned(t *testing.T) {
	data := []byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main"><conditionalFormatting sqref="A1:A2"><cfRule><dataBar/></cfRule><cfRule><dataBar/><extLst><ext><x14:id>second</x14:id></ext></extLst></cfRule></conditionalFormatting><extLst><ext><x14:cfRule id="second"><x14:dataBar><x14:cfvo type="autoMin"/><x14:cfvo type="autoMax"/></x14:dataBar></x14:cfRule></ext></extLst></worksheet>`)
	rules := []model.CFRule{{Type: "data_bar", Ranges: []model.Range{{R1: 1}}}, {Type: "data_bar", Ranges: []model.Range{{R1: 1}}, DataBar: &model.CFDataBar{Minimum: model.CFThreshold{Type: "num", Value: "0"}, Maximum: model.CFThreshold{Type: "num", Value: "100"}}}}
	got, err := replaceWorksheetBarBounds(data, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(`>100</xm:f>`)) {
		t.Fatalf("rule alignment lost: %s", got)
	}
	if _, err := replaceWorksheetBarBounds([]byte(`<worksheet>`), rules); err == nil {
		t.Fatal("malformed XML accepted")
	}
}
