package xlsxfile

import (
	"encoding/xml"
	"github.com/xuri/excelize/v2"
	"strings"
	"testing"
)

func TestExtendedConditionalRules(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	r := &reader{f: f}
	legacy := `<conditionalFormatting sqref="E3:E5"><cfRule type="dataBar" priority="6"><dataBar><cfvo type="num" val="0"/><cfvo type="num" val="1"/><color theme="5"/></dataBar><extLst><ext><id>bar-id</id></ext></extLst></cfRule></conditionalFormatting>`
	extended := `<x14:conditionalFormatting><x14:cfRule type="dataBar" id="bar-id"><x14:dataBar border="1" gradient="0"><x14:cfvo type="num"><xm:f>0</xm:f></x14:cfvo><x14:cfvo type="num"><xm:f>1</xm:f></x14:cfvo><x14:borderColor rgb="FFF1C94D"/></x14:dataBar></x14:cfRule><xm:sqref>E3:E5</xm:sqref></x14:conditionalFormatting>`
	icons := `<x14:conditionalFormatting><x14:cfRule type="iconSet" priority="41"><x14:iconSet custom="1"><x14:cfvo type="percent"><xm:f>0</xm:f></x14:cfvo><x14:cfvo type="num"><xm:f>0</xm:f></x14:cfvo><x14:cfvo type="num"><xm:f>1</xm:f></x14:cfvo><x14:cfIcon iconSet="NoIcons" iconId="0"/><x14:cfIcon iconSet="NoIcons" iconId="0"/><x14:cfIcon iconSet="4TrafficLights" iconId="0"/></x14:iconSet></x14:cfRule><xm:sqref>F3:F5</xm:sqref></x14:conditionalFormatting>`
	var raw []rawRule
	for _, source := range []string{legacy, extended, icons} {
		dec := xml.NewDecoder(strings.NewReader(source))
		token, err := dec.RawToken()
		if err != nil {
			t.Fatal(err)
		}
		var group rawConditionalGroup
		if err := decodeRawElement(dec, token.(xml.StartElement), &group); err != nil {
			t.Fatal(err)
		}
		raw = append(raw, r.readConditionalGroup(group)...)
	}
	rules := r.condFormats(mergeExtendedConditions(raw))
	if len(rules) != 2 {
		t.Fatalf("duplicate or missing rules: %+v", rules)
	}
	bar := rules[0]
	if bar.Type != "data_bar" || len(bar.Colors) != 1 || bar.Colors[0] == "" || bar.DataBar == nil || !bar.DataBar.Solid || bar.DataBar.BorderColor != "F1C94D" || bar.DataBar.Maximum.Value != "1" {
		t.Fatalf("bar settings lost: %+v", bar)
	}
	set := rules[1].IconSet
	if set == nil || len(set.Icons) != 3 || set.Icons[2].Set != "4TrafficLights" || set.Icons[2].Index != 0 || set.Thresholds[2].Value != "1" {
		t.Fatalf("custom icons lost: %+v", set)
	}
}

func TestExtendedDataBarPreservesUnspecifiedSettings(t *testing.T) {
	for _, tc := range []struct {
		name, legacy, extended string
		hidden                 bool
	}{
		{"hidden value inherited", `showValue="0"`, `gradient="0"`, true},
		{"visible value inherited", `showValue="1"`, `gradient="0"`, false},
		{"explicit visible override", `showValue="0"`, `gradient="0" showValue="1"`, false},
		{"explicit hidden override", `showValue="1"`, `gradient="0" showValue="0"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := excelize.NewFile()
			defer f.Close()
			r := &reader{f: f}
			sources := []string{
				`<conditionalFormatting sqref="A1:A2"><cfRule type="dataBar" priority="1"><dataBar ` + tc.legacy + `><cfvo type="num" val="0"/><cfvo type="num" val="100"/><color rgb="FF638EC6"/></dataBar><extLst><ext><id>bar-id</id></ext></extLst></cfRule></conditionalFormatting>`,
				`<conditionalFormatting sqref="A1:A2"><cfRule type="dataBar" id="bar-id"><dataBar ` + tc.extended + `/></cfRule></conditionalFormatting>`,
			}
			var raw []rawRule
			for _, source := range sources {
				var group rawConditionalGroup
				if err := xml.Unmarshal([]byte(source), &group); err != nil {
					t.Fatal(err)
				}
				raw = append(raw, r.readConditionalGroup(group)...)
			}
			merged := mergeExtendedConditions(raw)
			if len(merged) != 1 {
				t.Fatalf("rules: %+v", merged)
			}
			bar := merged[0].rule.DataBar
			if bar == nil || bar.HideValue != tc.hidden || !bar.Solid || bar.Minimum.Value != "0" || bar.Maximum.Value != "100" {
				t.Fatalf("merged bar: %+v, hidden want %v", bar, tc.hidden)
			}
			if raw[0].rule.DataBar.Solid {
				t.Fatal("merge mutated legacy input")
			}
		})
	}
}
