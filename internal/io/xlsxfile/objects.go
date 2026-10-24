package xlsxfile

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // decoders for picture sizes
	_ "image/jpeg"
	_ "image/png"
	"reflect"
	"strings"

	"github.com/xuri/excelize/v2"

	"ww-excel/internal/model"
)

// objects writes charts and pictures that changed. Objects at a cell
// whose drawing changed are removed from the file and written again
// (excelize deletes drawings by anchor cell).
func (w *writer) objects(sh *model.Sheet, old, cur []model.Object) error {
	if reflect.DeepEqual(old, cur) {
		return nil
	}
	cellOf := func(a model.Anchor) string { n, _ := excelize.CoordinatesToCellName(a.Col+1, a.Row+1); return n }
	byID := map[int]model.Object{}
	for _, o := range old {
		byID[o.ID] = o
	}
	dirty := map[string]bool{}
	for _, o := range cur {
		if prev, ok := byID[o.ID]; ok && reflect.DeepEqual(prev, o) {
			continue
		}
		dirty[cellOf(o.Anchor)] = true
		if prev, ok := byID[o.ID]; ok {
			dirty[cellOf(prev.Anchor)] = true
		}
	}
	curIDs := map[int]bool{}
	for _, o := range cur {
		curIDs[o.ID] = true
	}
	for _, o := range old {
		if !curIDs[o.ID] {
			dirty[cellOf(o.Anchor)] = true
		}
	}
	for cell := range dirty {
		if err := w.f.DeleteChart(sh.Name, cell); err != nil {
			return err
		}
		if err := w.f.DeletePicture(sh.Name, cell); err != nil {
			return err
		}
	}
	for _, o := range cur {
		if !dirty[cellOf(o.Anchor)] {
			continue
		}
		if err := w.addObject(sh, o, cellOf(o.Anchor)); err != nil {
			return err
		}
	}
	return nil
}

func (w *writer) addObject(sh *model.Sheet, o model.Object, cell string) error {
	format := excelize.GraphicOptions{OffsetX: int(o.Anchor.DX), OffsetY: int(o.Anchor.DY), Positioning: "oneCell"}
	switch {
	case o.Image != nil:
		cfg, _, err := image.DecodeConfig(bytes.NewReader(o.Image.Data))
		if err != nil || cfg.Width == 0 || cfg.Height == 0 {
			return nil // not a picture we can size; leave it out
		}
		format.ScaleX = o.W / float64(cfg.Width)
		format.ScaleY = o.H / float64(cfg.Height)
		ext := o.Image.Ext
		if ext == "" {
			ext = ".png"
		}
		return w.f.AddPictureFromBytes(sh.Name, cell, &excelize.Picture{Extension: ext, File: o.Image.Data, Format: &format})
	case o.Chart != nil:
		c := o.Chart
		xc := &excelize.Chart{
			Type:      chartType(c),
			Format:    format,
			Dimension: excelize.ChartDimension{Width: uint(max(o.W, 50)), Height: uint(max(o.H, 50))},
			Legend:    excelize.ChartLegend{Position: legendPos(c.Legend)},
		}
		if c.Title != "" {
			xc.Title = excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: c.Title}}}
		}
		for _, s := range c.Series {
			xs := excelize.ChartSeries{Name: s.Name, Categories: s.Categories, Values: s.Values}
			xc.Series = append(xc.Series, xs)
		}
		if err := w.f.AddChart(sh.Name, cell, xc); err != nil {
			return fmt.Errorf("圖表：%w", err)
		}
	}
	return nil
}

func chartType(c *model.Chart) excelize.ChartType {
	switch c.Type {
	case "bar":
		if c.Stacked {
			return excelize.BarStacked
		}
		return excelize.Bar
	case "line":
		return excelize.Line
	case "pie":
		return excelize.Pie
	case "scatter":
		return excelize.Scatter
	case "area":
		if c.Stacked {
			return excelize.AreaStacked
		}
		return excelize.Area
	}
	if c.Stacked {
		return excelize.ColStacked
	}
	return excelize.Col
}

func legendPos(p string) string {
	switch p {
	case "bottom", "top", "none":
		return p
	}
	return "right"
}

// Paper sizes by Excel's paperSize codes.
var paperCodes = map[string]int{"Letter": 1, "Legal": 5, "A3": 8, "A4": 9, "B5": 13}

// pageSetup writes print settings that changed.
func (w *writer) pageSetup(sh *model.Sheet, old, cur *model.PageSetup) error {
	if reflect.DeepEqual(old, cur) || cur == nil {
		return nil
	}
	ps := *cur
	size := paperCodes[ps.Paper]
	if size == 0 {
		size = 9
	}
	orient := "portrait"
	if ps.Landscape {
		orient = "landscape"
	}
	opts := &excelize.PageLayoutOptions{Size: &size, Orientation: &orient}
	if ps.FitWide > 0 || ps.FitTall > 0 {
		fw, ft := ps.FitWide, ps.FitTall
		opts.FitToWidth, opts.FitToHeight = &fw, &ft
	} else {
		scale := uint(ps.Scale)
		opts.AdjustTo = &scale
	}
	if err := w.f.SetPageLayout(sh.Name, opts); err != nil {
		return err
	}
	if err := w.f.SetPageMargins(sh.Name, &excelize.PageLayoutMarginsOptions{
		Top: &ps.Top, Bottom: &ps.Bottom, Left: &ps.Left, Right: &ps.Right, Header: &ps.Header, Footer: &ps.Footer,
		Horizontally: &ps.CenterH, Vertically: &ps.CenterV,
	}); err != nil {
		return err
	}
	hf := &excelize.HeaderFooterOptions{}
	if ps.HeaderText != "" {
		hf.OddHeader = "&C" + toExcelCodes(ps.HeaderText)
	}
	if ps.FooterText != "" {
		hf.OddFooter = "&C" + toExcelCodes(ps.FooterText)
	}
	if err := w.f.SetHeaderFooter(sh.Name, hf); err != nil {
		return err
	}
	// The print area is the sheet's _xlnm.Print_Area name.
	_ = w.f.DeleteDefinedName(&excelize.DefinedName{Name: "_xlnm.Print_Area", Scope: sh.Name})
	if a := ps.PrintArea; a != nil {
		if err := w.f.SetDefinedName(&excelize.DefinedName{Name: "_xlnm.Print_Area", Scope: sh.Name, RefersTo: quoteSheet(sh.Name) + "!" + absRef(*a)}); err != nil {
			return err
		}
	}
	return nil
}

func absRef(r model.Range) string {
	a, _ := excelize.CoordinatesToCellName(r.C0+1, r.R0+1, true)
	b, _ := excelize.CoordinatesToCellName(r.C1+1, r.R1+1, true)
	return a + ":" + b
}

func quoteSheet(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// toExcelCodes keeps our header codes (&P, &N, &D, &T, &F, &A), which
// are Excel's own.
func toExcelCodes(s string) string { return s }

// readPageSetup reads print settings; nil when the sheet has none.
func readPageSetup(f *excelize.File, sheet string, wb *model.Workbook) *model.PageSetup {
	ps := model.DefaultPageSetup()
	found := false
	if pl, err := f.GetPageLayout(sheet); err == nil {
		if pl.Size != nil {
			for name, code := range paperCodes {
				if code == *pl.Size {
					ps.Paper, found = name, true
				}
			}
		}
		if pl.Orientation != nil && *pl.Orientation == "landscape" {
			ps.Landscape, found = true, true
		}
		if pl.AdjustTo != nil && *pl.AdjustTo >= 10 && *pl.AdjustTo != 100 {
			ps.Scale, found = int(*pl.AdjustTo), true
		}
		if pl.FitToWidth != nil && *pl.FitToWidth > 0 || pl.FitToHeight != nil && *pl.FitToHeight > 0 {
			if pl.FitToWidth != nil {
				ps.FitWide = *pl.FitToWidth
			}
			if pl.FitToHeight != nil {
				ps.FitTall = *pl.FitToHeight
			}
		}
	}
	if m, err := f.GetPageMargins(sheet); err == nil {
		set := func(dst *float64, src *float64) {
			if src != nil {
				*dst = *src
			}
		}
		set(&ps.Top, m.Top)
		set(&ps.Bottom, m.Bottom)
		set(&ps.Left, m.Left)
		set(&ps.Right, m.Right)
		set(&ps.Header, m.Header)
		set(&ps.Footer, m.Footer)
		if m.Horizontally != nil {
			ps.CenterH = *m.Horizontally
		}
		if m.Vertically != nil {
			ps.CenterV = *m.Vertically
		}
	}
	if hf, err := f.GetHeaderFooter(sheet); err == nil && hf != nil {
		ps.HeaderText = centerSection(hf.OddHeader)
		ps.FooterText = centerSection(hf.OddFooter)
		found = found || hf.OddHeader != "" || hf.OddFooter != ""
	}
	for _, dn := range f.GetDefinedName() {
		if dn.Name == "_xlnm.Print_Area" && dn.Scope == sheet {
			ref := dn.RefersTo
			if i := strings.LastIndex(ref, "!"); i >= 0 {
				ref = ref[i+1:]
			}
			if rs := parseSqref(strings.Split(ref, ",")[0]); len(rs) == 1 {
				ps.PrintArea, found = &rs[0], true
			}
		}
	}
	if !found {
		return nil
	}
	return &ps
}

// centerSection returns the &C part of an Excel header/footer string
// (or the whole string when it has no sections).
func centerSection(s string) string {
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "&C"); i >= 0 {
		rest := s[i+2:]
		for _, sec := range []string{"&L", "&R"} {
			if j := strings.Index(rest, sec); j >= 0 {
				rest = rest[:j]
			}
		}
		return rest
	}
	if strings.HasPrefix(s, "&L") || strings.HasPrefix(s, "&R") {
		return ""
	}
	return s
}
