package xlsxfile

import (
	"encoding/xml"
	"fmt"
	"github.com/xuri/excelize/v2"
	"reflect"
	"strings"
	"ww-excel/internal/model"
)

// Read the underlying OOXML: Excelize's public Fill drops pattern background
// colors and arbitrary gradient stop positions and angles.
func readCellFill(f *excelize.File, id int, st *model.Style) {
	if f.Styles == nil || id < 0 || id >= len(f.Styles.CellXfs.Xf) {
		return
	}
	xf := f.Styles.CellXfs.Xf[id]
	if xf.FillID == nil || f.Styles.Fills == nil || *xf.FillID >= len(f.Styles.Fills.Fill) {
		return
	}
	fill := f.Styles.Fills.Fill[*xf.FillID]
	color := func(rgb string, index int, theme *int, tint float64, fallback string) string {
		c := normalizeHex(excelize.ThemeColor(f.GetBaseColor(rgb, index, theme), tint))
		if c == "" {
			return fallback
		}
		return c
	}
	if p := fill.PatternFill; p != nil && p.PatternType != "" && p.PatternType != "none" && p.PatternType != "solid" {
		spec := model.FillSpec{Kind: "pattern", Pattern: p.PatternType, Foreground: "000000", Background: "FFFFFF"}
		if c := p.FgColor; c != nil {
			spec.Foreground = color(c.RGB, c.Indexed, c.Theme, c.Tint, "000000")
		}
		if c := p.BgColor; c != nil {
			spec.Background = color(c.RGB, c.Indexed, c.Theme, c.Tint, "FFFFFF")
		}
		st.Fill = ""
		st.FillSpec = spec
	}
	if g := fill.GradientFill; g != nil {
		spec := model.FillSpec{Kind: "linear", Degree: g.Degree, Left: g.Left, Right: g.Right, Top: g.Top, Bottom: g.Bottom}
		if g.Type == "path" {
			spec.Kind = "path"
		}
		stops := []model.FillStop{}
		for _, s := range g.Stop {
			c := s.Color
			stops = append(stops, model.FillStop{Position: s.Position, Color: color(c.RGB, c.Indexed, c.Theme, c.Tint, "000000")})
		}
		spec.Stops = model.EncodeFillStops(stops)
		st.Fill = ""
		st.FillSpec = spec
	}
}

// Add the exact fill and xf without mutating an existing interned style.
func newCellStyle(f *excelize.File, st model.Style) (int, error) {
	xs := toExcelize(st)
	id, err := f.NewStyle(xs)
	if err != nil || st.FillSpec.Kind == "" {
		return id, err
	}
	spec, err := model.NormalizeFillSpec(st.FillSpec)
	if err != nil {
		return 0, err
	}
	var content string
	if spec.Kind == "pattern" {
		content = fmt.Sprintf(`<patternFill patternType="%s"><fgColor rgb="FF%s"/><bgColor rgb="FF%s"/></patternFill>`, spec.Pattern, spec.Foreground, spec.Background)
	} else {
		var b strings.Builder
		if spec.Kind == "path" {
			fmt.Fprintf(&b, `<gradientFill type="path" left="%g" right="%g" top="%g" bottom="%g">`, spec.Left, spec.Right, spec.Top, spec.Bottom)
		} else {
			fmt.Fprintf(&b, `<gradientFill degree="%g">`, spec.Degree)
		}
		for _, stop := range spec.ColorStops() {
			fmt.Fprintf(&b, `<stop position="%g"><color rgb="FF%s"/></stop>`, stop.Position, stop.Color)
		}
		b.WriteString(`</gradientFill>`)
		content = b.String()
	}
	fill := *f.Styles.Fills.Fill[0]
	fill.PatternFill = nil
	fill.GradientFill = nil
	if err = xml.Unmarshal([]byte("<fill>"+content+"</fill>"), &fill); err != nil {
		return 0, err
	}
	fillID := -1
	for i, v := range f.Styles.Fills.Fill {
		if reflect.DeepEqual(v, &fill) {
			fillID = i
			break
		}
	}
	if fillID < 0 {
		fillID = len(f.Styles.Fills.Fill)
		f.Styles.Fills.Fill = append(f.Styles.Fills.Fill, &fill)
		f.Styles.Fills.Count = len(f.Styles.Fills.Fill)
	}
	xf := f.Styles.CellXfs.Xf[id]
	xf.FillID = &fillID
	yes := true
	xf.ApplyFill = &yes
	for i, v := range f.Styles.CellXfs.Xf {
		if reflect.DeepEqual(v, xf) {
			return i, nil
		}
	}
	id = len(f.Styles.CellXfs.Xf)
	f.Styles.CellXfs.Xf = append(f.Styles.CellXfs.Xf, xf)
	f.Styles.CellXfs.Count = len(f.Styles.CellXfs.Xf)
	return id, nil
}
