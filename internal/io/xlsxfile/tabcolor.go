package xlsxfile

import (
	"encoding/xml"
	"github.com/xuri/excelize/v2"
	"math"
	"strconv"
)

// Resolve against the source workbook theme, including its custom indexed palette.
func (r *reader) sheetTabColor(node xml.StartElement) string {
	if attr(node, "auto") == "1" || attr(node, "auto") == "true" {
		return ""
	}
	rgb := normalizeHex(attr(node, "rgb"))
	themeText, indexedText := attr(node, "theme"), attr(node, "indexed")
	var theme *int
	if themeText != "" {
		index, err := strconv.Atoi(themeText)
		if err != nil || index < 0 || index > 11 {
			return ""
		}
		theme = &index
	}
	if rgb == "" && theme == nil && indexedText == "" {
		return ""
	}
	indexed := 0
	if indexedText != "" {
		index, err := strconv.Atoi(indexedText)
		if err != nil || index < 0 || index >= 64 {
			return ""
		}
		indexed = index
	}
	if attr(node, "rgb") != "" && rgb == "" {
		return ""
	}
	base := normalizeHex(r.f.GetBaseColor(rgb, indexed, theme))
	if theme != nil && *theme >= 10 {
		key := "hlink"
		if *theme == 11 {
			key = "folHlink"
		}
		base = drawingTheme(r.parts)[key]
	}
	if _, err := strconv.ParseUint(base, 16, 24); err != nil || len(base) != 6 {
		return ""
	}
	tint := 0.
	if raw := attr(node, "tint"); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < -1 || value > 1 {
			return ""
		}
		tint = value
	}
	return normalizeHex(excelize.ThemeColor(base, tint))
}
