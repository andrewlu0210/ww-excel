package xlsxfile

import "ww-excel/internal/model"

// Horizontal borders in a custom table style separate data rows. Keep their
// resolved color independently from the worksheet's direct cell formatting.
func (r *reader) tableRowBorder(name string) model.BorderLine {
	if r.tableBorders != nil {
		return r.tableBorders[name]
	}
	r.tableBorders = map[string]model.BorderLine{}
	var styles struct {
		Tables []struct {
			Name     string `xml:"name,attr"`
			Elements []struct {
				Type string `xml:"type,attr"`
				Dxf  int    `xml:"dxfId,attr"`
			} `xml:"tableStyleElement"`
		} `xml:"tableStyles>tableStyle"`
		Formats []struct {
			Horizontal struct {
				Style string `xml:"style,attr"`
				Color struct {
					RGB   string  `xml:"rgb,attr"`
					Theme *int    `xml:"theme,attr"`
					Tint  float64 `xml:"tint,attr"`
				} `xml:"color"`
			} `xml:"border>horizontal"`
		} `xml:"dxfs>dxf"`
	}
	if decodePart(r.parts, "xl/styles.xml", &styles) != nil {
		return model.BorderLine{}
	}
	for _, table := range styles.Tables {
		for _, element := range table.Elements {
			if element.Type != "wholeTable" || element.Dxf < 0 || element.Dxf >= len(styles.Formats) {
				continue
			}
			line := styles.Formats[element.Dxf].Horizontal
			r.tableBorders[table.Name] = model.BorderLine{Style: line.Style, Color: resolveColor(r.f, line.Color.RGB, line.Color.Theme, line.Color.Tint)}
		}
	}
	return r.tableBorders[name]
}
