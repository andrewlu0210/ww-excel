package service

import (
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

// sheetLook renders cells with what lies on top of their own style:
// table styles and conditional formats. Built once per request.
type sheetLook struct {
	wb     *model.Workbook
	sh     *model.Sheet
	cf     *document.CFEval
	tables []model.Table
	// Interned results, keyed by base style and overlay.
	cache map[lookKey]uint32
}

type lookKey struct {
	base uint32
	over model.CFFormat
	head bool
}

// Table style colors: header fill and banded row fill.
type tablePalette struct{ header, band, line string }

var tablePalettes = map[string]tablePalette{
	"TableStyleMedium2":  {"4472C4", "D9E1F2", "8EA9DB"},
	"TableStyleMedium3":  {"ED7D31", "FCE4D6", "F4B084"},
	"TableStyleMedium4":  {"A5A5A5", "EDEDED", "C9C9C9"},
	"TableStyleMedium5":  {"FFC000", "FFF2CC", "FFD966"},
	"TableStyleMedium6":  {"5B9BD5", "DDEBF7", "9BC2E6"},
	"TableStyleMedium7":  {"70AD47", "E2EFDA", "A9D08E"},
	"TableStyleMedium9":  {"4472C4", "D9E1F2", "8EA9DB"},
	"TableStyleMedium1":  {"000000", "D9D9D9", "808080"},
	"TableStyleLight9":   {"4472C4", "FFFFFF", "4472C4"},
	"TableStyleLight1":   {"FFFFFF", "D9D9D9", "000000"},
	"TableStyleMedium16": {"4472C4", "D9E1F2", "8EA9DB"},
}

// TableStyles lists the table styles offered in the UI, in order.
var TableStyles = []string{"TableStyleMedium2", "TableStyleMedium7", "TableStyleMedium3", "TableStyleMedium5", "TableStyleMedium6", "TableStyleMedium4", "TableStyleMedium1", "TableStyleLight9"}

func paletteFor(style string) tablePalette {
	if p, ok := tablePalettes[style]; ok {
		return p
	}
	return tablePalettes["TableStyleMedium2"]
}

func newLook(d *document.Document, sheet int) *sheetLook {
	wb := d.Workbook()
	if sheet < 0 || sheet >= len(wb.Sheets) {
		return nil
	}
	sh := wb.Sheets[sheet]
	l := &sheetLook{wb: wb, sh: sh, tables: sh.Layout.Tables, cache: map[lookKey]uint32{}}
	l.cf = d.CondFormats(sheet, func(sh *model.Sheet, row, col int, c model.Cell) string {
		v, _, _, _, _ := cellView(wb, sh, row, col, c)
		return v
	})
	if l.cf == nil && len(l.tables) == 0 {
		return nil
	}
	return l
}

// areas are the ranges whose blank cells may still be formatted.
func (l *sheetLook) areas() []model.Range {
	var out []model.Range
	if l.cf != nil {
		out = append(out, l.cf.Ranges()...)
	}
	for _, t := range l.tables {
		out = append(out, t.Range)
	}
	return out
}

// style returns the style ID to show at a cell, with a data bar.
func (l *sheetLook) style(row, col int, c model.Cell, base uint32) (st uint32, bar float64, barColor string, any bool) {
	var over model.CFFormat
	head := false
	for _, t := range l.tables {
		if !t.Range.Contains(row, col) {
			continue
		}
		p := paletteFor(t.Style)
		switch {
		case row == t.Range.R0:
			over = model.CFFormat{Fill: p.header, Color: "FFFFFF", Bold: true}
			head = true
		case t.TotalsRow && row == t.Range.R1:
			over = model.CFFormat{Bold: true}
		case !t.NoBands && (row-t.Range.R0)%2 == 1:
			over = model.CFFormat{Fill: p.band}
		}
		any = true
		break
	}
	bar = -1
	if l.cf != nil {
		if res, ok := l.cf.At(row, col, c); ok {
			any = true
			cf := res.Format
			if res.ScaleFill != "" {
				cf.Fill = res.ScaleFill
			}
			// Conditional formats win over the table style.
			if cf.Fill != "" {
				over.Fill = cf.Fill
			}
			if cf.Color != "" {
				over.Color = cf.Color
			}
			over.Bold = over.Bold || cf.Bold
			over.Italic = over.Italic || cf.Italic
			over.Underline = over.Underline || cf.Underline
			over.Strike = over.Strike || cf.Strike
			bar, barColor = res.Bar, res.BarColor
		}
	}
	if !any || over == (model.CFFormat{}) {
		return base, bar, barColor, any
	}
	key := lookKey{base: base, over: over, head: head}
	if id, ok := l.cache[key]; ok {
		return id, bar, barColor, true
	}
	s := l.wb.StyleTable().Get(base)
	// Table styles sit under direct formatting; conditional formats
	// and header colors are applied over it.
	if over.Fill != "" && (s.Fill == "" || !head) {
		s.Fill = over.Fill
	}
	if over.Color != "" {
		s.Font.Color = over.Color
	}
	s.Font.Bold = s.Font.Bold || over.Bold
	s.Font.Italic = s.Font.Italic || over.Italic
	s.Font.Underline = s.Font.Underline || over.Underline
	s.Font.Strike = s.Font.Strike || over.Strike
	id := l.wb.StyleTable().Intern(s)
	l.cache[key] = id
	return id, bar, barColor, true
}

// view renders a cell like cellView, plus conditional formats and table
// styles when look is not nil.
func (l *sheetLook) view(row, col int, c model.Cell) (dto CellDTO, show bool) {
	v, n, x, st, fc := cellView(l.wb, l.sh, row, col, c)
	dto = CellDTO{V: v, N: n, X: x, St: st, Fc: fc}
	show = c.HasContent() || st != 0
	st2, bar, bc, any := l.style(row, col, c, st)
	if any {
		dto.St = st2
		if bar >= 0 && c.Kind == model.KindNumber {
			dto.Bar, dto.Bc = bar, bc
		}
		show = show || st2 != st
		// The style may change the display (a CF color does not change
		// the number format, so v stays).
	}
	return dto, show
}
