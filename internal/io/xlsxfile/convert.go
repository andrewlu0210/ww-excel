// Package xlsxfile reads and writes Excel workbooks with excelize.
//
// Saving updates the file the workbook was loaded from in place and only
// rewrites what changed, so content this app does not model yet (charts,
// images, data validation, comments, conditional formats...) survives.
package xlsxfile

import (
	"math"
	"strings"

	"github.com/xuri/excelize/v2"

	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// Excel's default column width unit is the width of a digit in the
// default font (7px for Calibri 11); row heights are points.
const digitPx = 7

func widthToPx(w float64) float64  { return math.Round(w * digitPx) }
func pxToWidth(px float64) float64 { return px / digitPx }
func ptToPx(pt float64) float64    { return math.Round(pt * 4 / 3) }
func pxToPt(px float64) float64    { return px * 3 / 4 }

var borderStyles = []string{
	"", "thin", "medium", "dashed", "dotted", "thick", "double", "hair",
	"mediumDashed", "dashDot", "mediumDashDot", "dashDotDot", "mediumDashDotDot", "slantDashDot",
}

func borderIndex(style string) int {
	for i, s := range borderStyles {
		if s == style {
			return i
		}
	}
	return 1
}

// fromExcelize converts an excelize style to ours. Theme colors are
// resolved to RGB. Black text is stored as automatic so it stays
// readable in dark mode.
func fromExcelize(f *excelize.File, xs *excelize.Style) model.Style {
	var st model.Style
	if xs == nil {
		return st
	}
	if fnt := xs.Font; fnt != nil {
		st.Font = model.Font{
			Name:      fnt.Family,
			Size:      fnt.Size,
			Bold:      fnt.Bold,
			Italic:    fnt.Italic,
			Underline: fnt.Underline != "" && fnt.Underline != "none",
			Strike:    fnt.Strike,
			Color:     resolveColor(f, fnt.Color, fnt.ColorTheme, fnt.ColorTint),
		}
		if st.Font.Color == "000000" {
			st.Font.Color = ""
		}
	}
	if fill := xs.Fill; (fill.Type == "pattern" && fill.Pattern > 0 || fill.Type == "gradient") && len(fill.Color) > 0 {
		st.Fill = normalizeHex(fill.Color[0])
	}
	for _, b := range xs.Border {
		if b.Style <= 0 || b.Style >= len(borderStyles) {
			continue
		}
		line := model.BorderLine{Style: borderStyles[b.Style], Color: normalizeHex(b.Color)}
		if line.Color == "000000" {
			line.Color = ""
		}
		switch b.Type {
		case "left":
			st.Border.Left = line
		case "right":
			st.Border.Right = line
		case "top":
			st.Border.Top = line
		case "bottom":
			st.Border.Bottom = line
		}
	}
	if a := xs.Alignment; a != nil {
		st.Align = model.Align{H: a.Horizontal, V: a.Vertical, Wrap: a.WrapText, Indent: a.Indent}
		if st.Align.H == "general" {
			st.Align.H = ""
		}
		if st.Align.V == "bottom" {
			st.Align.V = ""
		}
	}
	if xs.CustomNumFmt != nil {
		st.NumFmt = *xs.CustomNumFmt
	} else if code, ok := numfmt.Builtin(xs.NumFmt); ok {
		st.NumFmt = code
	}
	if strings.EqualFold(st.NumFmt, "General") {
		st.NumFmt = ""
	}
	return st
}

func resolveColor(f *excelize.File, hex string, theme *int, tint float64) string {
	if hex == "" && theme == nil {
		return ""
	}
	base := f.GetBaseColor(hex, 0, theme)
	if base == "" {
		return ""
	}
	return normalizeHex(excelize.ThemeColor(normalizeHex(base), tint))
}

// normalizeHex turns "FFRRGGBB" / "#RRGGBB" into "RRGGBB".
func normalizeHex(s string) string {
	s = strings.ToUpper(strings.TrimPrefix(s, "#"))
	if len(s) == 8 {
		s = s[2:]
	}
	if len(s) != 6 {
		return ""
	}
	return s
}

func toExcelize(st model.Style) *excelize.Style {
	xs := &excelize.Style{
		Font: &excelize.Font{
			Family: st.Font.Name,
			Size:   st.Font.Size,
			Bold:   st.Font.Bold,
			Italic: st.Font.Italic,
			Strike: st.Font.Strike,
			Color:  st.Font.Color,
		},
	}
	if st.Font.Underline {
		xs.Font.Underline = "single"
	}
	if st.Fill != "" {
		xs.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{st.Fill}}
	}
	sides := []struct {
		name string
		line model.BorderLine
	}{{"left", st.Border.Left}, {"right", st.Border.Right}, {"top", st.Border.Top}, {"bottom", st.Border.Bottom}}
	for _, s := range sides {
		if s.line.Style == "" {
			continue
		}
		color := s.line.Color
		if color == "" {
			color = "000000"
		}
		xs.Border = append(xs.Border, excelize.Border{Type: s.name, Color: color, Style: borderIndex(s.line.Style)})
	}
	if st.Align != (model.Align{}) {
		xs.Alignment = &excelize.Alignment{Horizontal: st.Align.H, Vertical: st.Align.V, WrapText: st.Align.Wrap, Indent: st.Align.Indent}
	}
	if st.NumFmt != "" {
		if id, ok := numfmt.BuiltinID(st.NumFmt); ok {
			xs.NumFmt = id
		} else {
			code := st.NumFmt
			xs.CustomNumFmt = &code
		}
	}
	return xs
}
