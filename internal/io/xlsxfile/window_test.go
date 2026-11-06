package xlsxfile

import (
	"reflect"
	"strings"
	"testing"

	"ww-excel/internal/model"
)

func TestParseWindowView(t *testing.T) {
	for _, tc := range []struct {
		name, views string
		want        model.WindowView
	}{
		{"test1", `<workbookView xWindow="1440" yWindow="660" windowWidth="27240" windowHeight="17660"/>`, model.WindowView{Width: 1362, Height: 883, Position: &model.WindowPosition{X: 72, Y: 33}}},
		{"negative position", `<workbookView xWindow="-28800" yWindow="-1200" windowWidth="16000" windowHeight="12000"/>`, model.WindowView{Width: 800, Height: 600, Position: &model.WindowPosition{X: -1440, Y: -60}}},
		{"zero position", `<workbookView xWindow="0" yWindow="0"/>`, model.WindowView{Position: &model.WindowPosition{}}},
		{"position only", `<workbookView xWindow="1440" yWindow="660"/>`, model.WindowView{Position: &model.WindowPosition{X: 72, Y: 33}}},
		{"negative rounding", `<workbookView xWindow="-31" yWindow="-29"/>`, model.WindowView{Position: &model.WindowPosition{X: -2, Y: -1}}},
		{"invalid position preserves size", `<workbookView xWindow="abc" yWindow="660" windowWidth="16000" windowHeight="12000"/>`, model.WindowView{Width: 800, Height: 600}},
		{"partial position", `<workbookView xWindow="1440" windowWidth="16000" windowHeight="12000"/>`, model.WindowView{Width: 800, Height: 600}},
		{"position overflow", `<workbookView xWindow="2147483648" yWindow="660"/>`, model.WindowView{}},
		{"round", `<workbookView windowWidth="16011" windowHeight="12009"/>`, model.WindowView{Width: 801, Height: 600}},
		{"first visible", `<workbookView visibility="hidden" windowWidth="8000" windowHeight="8000"/><workbookView windowWidth="16000" windowHeight="12000"/>`, model.WindowView{Width: 800, Height: 600}},
		{"first view", `<workbookView windowWidth="16000" windowHeight="12000"/><workbookView windowWidth="20000" windowHeight="14000"/>`, model.WindowView{Width: 800, Height: 600}},
		{"missing", ``, model.WindowView{}},
		{"partial", `<workbookView windowWidth="16000"/>`, model.WindowView{}},
		{"zero", `<workbookView windowWidth="0" windowHeight="12000"/>`, model.WindowView{}},
		{"negative", `<workbookView windowWidth="-16000" windowHeight="12000"/>`, model.WindowView{}},
		{"overflow", `<workbookView windowWidth="99999999999999999999" windowHeight="12000"/>`, model.WindowView{}},
		{"invalid", `<workbookView windowWidth="abc" windowHeight="12000"/>`, model.WindowView{}},
		{"broken xml", `<workbookView`, model.WindowView{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xml := `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><bookViews>` + tc.views + `</bookViews></workbook>`
			if got := parseWindowView(strings.NewReader(xml)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
