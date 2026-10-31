package xlsxfile

import (
	"archive/zip"
	"github.com/xuri/excelize/v2"
	"ww-excel/internal/model"
)

// Column units use the Normal style's font, which need not match cell XF 0.
func normalColumnFont(parts map[string]*zip.File, fallback model.Font) model.Font {
	var styles struct {
		Fonts []struct {
			Name struct {
				Value string `xml:"val,attr"`
			} `xml:"name"`
			Size struct {
				Value float64 `xml:"val,attr"`
			} `xml:"sz"`
			Bold *struct {
				Value string `xml:"val,attr"`
			} `xml:"b"`
			Italic *struct {
				Value string `xml:"val,attr"`
			} `xml:"i"`
		} `xml:"fonts>font"`
		XFs []struct {
			Font int `xml:"fontId,attr"`
		} `xml:"cellStyleXfs>xf"`
		Styles []struct {
			Builtin *int `xml:"builtinId,attr"`
			XF      int  `xml:"xfId,attr"`
		} `xml:"cellStyles>cellStyle"`
	}
	if decodePart(parts, "xl/styles.xml", &styles) != nil {
		return fallback
	}
	for _, style := range styles.Styles {
		if style.Builtin == nil || *style.Builtin != 0 || style.XF < 0 || style.XF >= len(styles.XFs) {
			continue
		}
		index := styles.XFs[style.XF].Font
		if index < 0 || index >= len(styles.Fonts) {
			continue
		}
		font := styles.Fonts[index]
		return model.Font{Name: font.Name.Value, Size: font.Size.Value, Bold: font.Bold != nil && font.Bold.Value != "0" && font.Bold.Value != "false", Italic: font.Italic != nil && font.Italic.Value != "0" && font.Italic.Value != "false"}
	}
	return fallback
}

// A source-less export may use a different Normal font from the imported
// workbook. Convert pixel widths using the font actually written to the file.
func outputColumnFont(f *excelize.File, fallback model.Font) model.Font {
	styles := f.Styles
	if styles == nil || styles.CellStyles == nil || styles.CellStyleXfs == nil || styles.Fonts == nil {
		return fallback
	}
	for _, style := range styles.CellStyles.CellStyle {
		if style == nil || style.BuiltInID == nil || *style.BuiltInID != 0 || style.XfID < 0 || style.XfID >= len(styles.CellStyleXfs.Xf) {
			continue
		}
		xf := styles.CellStyleXfs.Xf[style.XfID]
		index := 0
		if xf.FontID != nil {
			index = *xf.FontID
		}
		if index < 0 || index >= len(styles.Fonts.Font) || styles.Fonts.Font[index] == nil {
			continue
		}
		source := styles.Fonts.Font[index]
		font := model.Font{}
		if source.Name != nil && source.Name.Val != nil {
			font.Name = *source.Name.Val
		}
		if source.Sz != nil && source.Sz.Val != nil {
			font.Size = *source.Sz.Val
		}
		font.Bold = source.B != nil && (source.B.Val == nil || *source.B.Val)
		font.Italic = source.I != nil && (source.I.Val == nil || *source.I.Val)
		return font
	}
	return fallback
}
