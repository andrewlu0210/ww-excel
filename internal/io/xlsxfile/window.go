package xlsxfile

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"math"
	"strconv"

	"ww-excel/internal/model"
)

func readWindowView(parts map[string]*zip.File) model.WindowView {
	p := parts["xl/workbook.xml"]
	if p == nil {
		return model.WindowView{}
	}
	r, err := p.Open()
	if err != nil {
		return model.WindowView{}
	}
	defer r.Close()
	return parseWindowView(r)
}

func parseWindowView(r io.Reader) model.WindowView {
	var book struct {
		Views []struct {
			X          string `xml:"xWindow,attr"`
			Y          string `xml:"yWindow,attr"`
			Width      string `xml:"windowWidth,attr"`
			Height     string `xml:"windowHeight,attr"`
			Visibility string `xml:"visibility,attr"`
		} `xml:"bookViews>workbookView"`
	}
	if xml.NewDecoder(r).Decode(&book) != nil {
		return model.WindowView{}
	}
	for _, view := range book.Views {
		if view.Visibility != "" && view.Visibility != "visible" {
			continue
		}
		result := model.WindowView{}
		width, we := strconv.ParseUint(view.Width, 10, 32)
		height, he := strconv.ParseUint(view.Height, 10, 32)
		// A twip is 1/20 point; Mac Excel uses logical points for window geometry.
		if we == nil && he == nil && width > 0 && height > 0 {
			result.Width, result.Height = int((width+10)/20), int((height+10)/20)
		}
		x, xe := strconv.ParseInt(view.X, 10, 32)
		y, ye := strconv.ParseInt(view.Y, 10, 32)
		if xe == nil && ye == nil {
			result.Position = &model.WindowPosition{X: int(math.Round(float64(x) / 20)), Y: int(math.Round(float64(y) / 20))}
		}
		return result
	}
	return model.WindowView{}
}
