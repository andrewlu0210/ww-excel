package xlsxfile

import "ww-excel/internal/model"

type tableFontBool struct {
	Val string `xml:"val,attr"`
}
type tableFontColor struct {
	RGB   string  `xml:"rgb,attr"`
	Theme *int    `xml:"theme,attr"`
	Tint  float64 `xml:"tint,attr"`
	Auto  bool    `xml:"auto,attr"`
}
type rawTableFont struct {
	Name *struct {
		Val string `xml:"val,attr"`
	} `xml:"name"`
	Size *struct {
		Val float64 `xml:"val,attr"`
	} `xml:"sz"`
	Color     *tableFontColor `xml:"color"`
	Bold      *tableFontBool  `xml:"b"`
	Italic    *tableFontBool  `xml:"i"`
	Strike    *tableFontBool  `xml:"strike"`
	Underline *tableFontBool  `xml:"u"`
}

func (r *reader) tableFont(font *rawTableFont) *model.RichFont {
	if font == nil {
		return nil
	}
	out := &model.RichFont{}
	boolean := func(v *tableFontBool) *bool {
		if v == nil {
			return nil
		}
		value := v.Val != "0" && v.Val != "false"
		return &value
	}
	out.Bold = boolean(font.Bold)
	out.Italic = boolean(font.Italic)
	out.Strike = boolean(font.Strike)
	if font.Name != nil {
		out.Name = &font.Name.Val
	}
	if font.Size != nil {
		out.Size = &font.Size.Val
	}
	if font.Underline != nil {
		value := font.Underline.Val != "none"
		out.Underline = &value
	}
	if c := font.Color; c != nil {
		color := resolveColor(r.f, c.RGB, c.Theme, c.Tint)
		if c.Auto {
			color = ""
		}
		out.Color = &color
	}
	return out
}

// Custom table fonts are overlays, separate from the cell's stored style.
func (r *reader) readTableStyles() {
	if r.tableBorders != nil {
		return
	}
	r.tableBorders = map[string]model.BorderLine{}
	r.tableAppearances = map[string]*model.TableAppearance{}
	type rawBorder struct {
		Style string         `xml:"style,attr"`
		Color tableFontColor `xml:"color"`
	}
	var styles struct {
		Tables []struct {
			Name     string `xml:"name,attr"`
			Elements []struct {
				Type string `xml:"type,attr"`
				Dxf  int    `xml:"dxfId,attr"`
			} `xml:"tableStyleElement"`
		} `xml:"tableStyles>tableStyle"`
		Formats []struct {
			Font   *rawTableFont `xml:"font"`
			Border struct {
				Horizontal rawBorder `xml:"horizontal"`
				Bottom     rawBorder `xml:"bottom"`
			} `xml:"border"`
		} `xml:"dxfs>dxf"`
	}
	if decodePart(r.parts, "xl/styles.xml", &styles) != nil {
		return
	}
	border := func(b rawBorder) model.BorderLine {
		return model.BorderLine{Style: b.Style, Color: resolveColor(r.f, b.Color.RGB, b.Color.Theme, b.Color.Tint)}
	}
	for _, table := range styles.Tables {
		appearance := &model.TableAppearance{}
		for _, element := range table.Elements {
			if element.Dxf < 0 || element.Dxf >= len(styles.Formats) {
				continue
			}
			format := styles.Formats[element.Dxf]
			font := r.tableFont(format.Font)
			switch element.Type {
			case "wholeTable":
				appearance.Whole = font
				r.tableBorders[table.Name] = border(format.Border.Horizontal)
			case "headerRow":
				appearance.Header = font
				appearance.HeaderBorder = border(format.Border.Bottom)
			case "firstColumn":
				appearance.First = font
			case "lastColumn":
				appearance.Last = font
			case "totalRow":
				appearance.Totals = font
			}
		}
		r.tableAppearances[table.Name] = appearance
	}
}
func (r *reader) tableRowBorder(name string) model.BorderLine {
	r.readTableStyles()
	return r.tableBorders[name]
}
func (r *reader) tableAppearance(name string) *model.TableAppearance {
	r.readTableStyles()
	return r.tableAppearances[name]
}
