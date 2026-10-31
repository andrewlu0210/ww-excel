package xlsxfile

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/model"
)

type drawingPercent struct {
	Val float64 `xml:"val,attr"`
}
type drawingColor struct {
	Val    string          `xml:"val,attr"`
	Last   string          `xml:"lastClr,attr"`
	LumMod *drawingPercent `xml:"lumMod"`
	LumOff *drawingPercent `xml:"lumOff"`
	Tint   *drawingPercent `xml:"tint"`
	Shade  *drawingPercent `xml:"shade"`
	SatMod *drawingPercent `xml:"satMod"`
	Alpha  *drawingPercent `xml:"alpha"`
}
type drawingFill struct {
	RGB    *drawingColor `xml:"srgbClr"`
	Scheme *drawingColor `xml:"schemeClr"`
	System *drawingColor `xml:"sysClr"`
}
type drawingFont struct {
	Size      string      `xml:"sz,attr"`
	Bold      string      `xml:"b,attr"`
	Italic    string      `xml:"i,attr"`
	Underline string      `xml:"u,attr"`
	Strike    string      `xml:"strike,attr"`
	Fill      drawingFill `xml:"solidFill"`
	Latin     struct {
		Name string `xml:"typeface,attr"`
	} `xml:"latin"`
	EastAsian struct {
		Name string `xml:"typeface,attr"`
	} `xml:"ea"`
}
type drawingShape struct {
	Properties struct {
		Transform struct {
			Ext struct {
				CX float64 `xml:"cx,attr"`
				CY float64 `xml:"cy,attr"`
			} `xml:"ext"`
		} `xml:"xfrm"`
		Geometry struct {
			Type        string `xml:"prst,attr"`
			Adjustments []struct {
				Name    string `xml:"name,attr"`
				Formula string `xml:"fmla,attr"`
			} `xml:"avLst>gd"`
		} `xml:"prstGeom"`
		Fill drawingFill `xml:"solidFill"`
		Line struct {
			Width  float64     `xml:"w,attr"`
			NoFill *struct{}   `xml:"noFill"`
			Fill   drawingFill `xml:"solidFill"`
		} `xml:"ln"`
	} `xml:"spPr"`
	Body struct {
		Properties struct {
			Anchor string `xml:"anchor,attr"`
			Left   string `xml:"lIns,attr"`
			Right  string `xml:"rIns,attr"`
			Top    string `xml:"tIns,attr"`
			Bottom string `xml:"bIns,attr"`
		} `xml:"bodyPr"`
		Paragraphs []struct {
			Properties struct {
				Align string      `xml:"algn,attr"`
				Font  drawingFont `xml:"defRPr"`
			} `xml:"pPr"`
			Runs []struct {
				Font drawingFont `xml:"rPr"`
				Text string      `xml:"t"`
			} `xml:"r"`
		} `xml:"p"`
	} `xml:"txBody"`
}

func drawingTheme(parts map[string]*zip.File) map[string]string {
	name := relOfType(parts, "xl/workbook.xml", "/theme")
	if name == "" {
		name = "xl/theme/theme1.xml"
	}
	var theme struct {
		Colors struct {
			Entries []struct {
				XMLName xml.Name
				RGB     drawingColor `xml:"srgbClr"`
				System  drawingColor `xml:"sysClr"`
			} `xml:",any"`
		} `xml:"themeElements>clrScheme"`
	}
	out := map[string]string{}
	if decodePart(parts, name, &theme) == nil {
		for _, entry := range theme.Colors.Entries {
			value := entry.RGB.Val
			if value == "" {
				value = entry.System.Last
			}
			out[entry.XMLName.Local] = normalizeHex(value)
		}
	}
	for alias, key := range map[string]string{"tx1": "dk1", "bg1": "lt1", "tx2": "dk2", "bg2": "lt2"} {
		out[alias] = out[key]
	}
	return out
}

func (fill drawingFill) color(theme map[string]string) string {
	c, base := fill.RGB, ""
	if c != nil {
		base = c.Val
	} else if c = fill.Scheme; c != nil {
		base = theme[c.Val]
	} else if c = fill.System; c != nil {
		base = c.Last
	}
	base = normalizeHex(base)
	if c == nil || len(base) != 6 {
		return ""
	}
	if c.LumMod != nil || c.LumOff != nil {
		n, _ := strconv.ParseUint(base, 16, 32)
		r, g, b := float64(n>>16&255)/255, float64(n>>8&255)/255, float64(n&255)/255
		light := (math.Max(r, math.Max(g, b)) + math.Min(r, math.Min(g, b))) / 2
		mod, off := 1.0, 0.0
		if c.LumMod != nil {
			mod = c.LumMod.Val / 100000
		}
		if c.LumOff != nil {
			off = c.LumOff.Val / 100000
		}
		next := math.Max(0, math.Min(1, light*mod+off))
		tint := 0.0
		if next > light && light < 1 {
			tint = (next - light) / (1 - light)
		} else if light > 0 {
			tint = next/light - 1
		}
		base = normalizeHex(excelize.ThemeColor(base, tint))
	}
	if c.SatMod != nil {
		n, _ := strconv.ParseUint(base, 16, 32)
		r, g, b := float64(n>>16&255)/255, float64(n>>8&255)/255, float64(n&255)/255
		lo, hi := math.Min(r, math.Min(g, b)), math.Max(r, math.Max(g, b))
		light := (hi + lo) / 2
		if hi > lo {
			saturation := (hi - lo) / (1 - math.Abs(2*light-1))
			target := math.Max(0, math.Min(1, saturation*c.SatMod.Val/100000))
			factor := target / saturation
			base = colorRGB(light+(r-light)*factor, light+(g-light)*factor, light+(b-light)*factor)
		}
	}
	if c.Tint != nil || c.Shade != nil {
		n, _ := strconv.ParseUint(base, 16, 32)
		channels := []float64{float64(n>>16&255) / 255, float64(n>>8&255) / 255, float64(n&255) / 255}
		// DrawingML tint and shade mix intensities in linear RGB.
		for i, v := range channels {
			v = srgbLinear(v)
			if c.Tint != nil {
				factor := math.Max(0, math.Min(1, c.Tint.Val/100000))
				v = v*factor + 1 - factor
			}
			if c.Shade != nil {
				v *= math.Max(0, math.Min(1, c.Shade.Val/100000))
			}
			channels[i] = linearSRGB(v)
		}
		base = colorRGB(channels[0], channels[1], channels[2])
	}

	return base
}

func (f drawingFont) rich(theme map[string]string) *model.RichFont {
	out := &model.RichFont{}
	name := f.EastAsian.Name
	if name == "" {
		name = f.Latin.Name
	}
	if name != "" {
		out.Name = &name
	}
	if f.Size != "" {
		size := xmlFloat(f.Size) / 100
		out.Size = &size
	}
	if f.Bold != "" {
		value := f.Bold == "1" || f.Bold == "true"
		out.Bold = &value
	}
	if f.Italic != "" {
		value := f.Italic == "1" || f.Italic == "true"
		out.Italic = &value
	}
	if f.Underline != "" {
		value := f.Underline != "none"
		out.Underline = &value
	}
	if f.Strike != "" {
		value := f.Strike != "noStrike"
		out.Strike = &value
	}
	if color := f.Fill.color(theme); color != "" {
		out.Color = &color
	}
	return out
}

func (raw *drawingShape) model(theme map[string]string, width, height float64) *model.Shape {
	kind := raw.Properties.Geometry.Type
	if kind != "" && kind != "rect" && kind != "roundRect" {
		return nil
	}
	body := raw.Body.Properties
	pad := func(value string, fallback float64) float64 {
		if value == "" {
			return fallback
		}
		return xmlFloat(value) / emuPerPx
	}
	shape := &model.Shape{Geometry: kind, Fill: raw.Properties.Fill.color(theme), Font: model.DefaultFont, Text: &model.RichText{}, AlignH: "left", AlignV: "top",
		Left: pad(body.Left, 9.6), Right: pad(body.Right, 9.6), Top: pad(body.Top, 4.8), Bottom: pad(body.Bottom, 4.8)}
	if body.Anchor == "ctr" {
		shape.AlignV = "center"
	}
	if body.Anchor == "b" {
		shape.AlignV = "bottom"
	}
	if raw.Properties.Line.NoFill == nil {
		shape.LineColor = raw.Properties.Line.Fill.color(theme)
		if shape.LineColor != "" {
			shape.LineWidth = raw.Properties.Line.Width / emuPerPx
			if shape.LineWidth == 0 {
				shape.LineWidth = 1
			}
		}
	}
	if kind == "roundRect" {
		adj := 16667.0
		for _, item := range raw.Properties.Geometry.Adjustments {
			if item.Name == "adj" {
				adj = xmlFloat(strings.TrimPrefix(item.Formula, "val "))
			}
		}
		shape.Radius = math.Min(width, height) * adj / 100000
	}
	for i, para := range raw.Body.Paragraphs {
		if i > 0 {
			shape.Text.Runs = append(shape.Text.Runs, model.TextRun{Text: "\n"})
		}
		if para.Properties.Align == "ctr" {
			shape.AlignH = "center"
		} else if para.Properties.Align == "r" {
			shape.AlignH = "right"
		}
		base := para.Properties.Font.rich(theme).Resolve(shape.Font)
		for _, run := range para.Runs {
			shape.Text.Runs = append(shape.Text.Runs, model.TextRun{Text: run.Text, Font: model.RichFontOf(run.Font.rich(theme).Resolve(base))})
		}
	}
	if len(shape.Text.Runs) == 0 && shape.Fill == "" {
		return nil
	}
	return shape
}

func colorRGB(r, g, b float64) string {
	channel := func(v float64) int { return int(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	return fmt.Sprintf("%02X%02X%02X", channel(r), channel(g), channel(b))
}

func srgbLinear(v float64) float64 {
	if v <= .04045 {
		return v / 12.92
	}
	return math.Pow((v+.055)/1.055, 2.4)
}
func linearSRGB(v float64) float64 {
	if v <= .0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - .055
}
