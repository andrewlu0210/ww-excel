package service

import (
	"context"
	"sort"
	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

// SelectionFormatDTO contains editable base values and per-property mixed flags.
// Border values describe the selected ranges, including their internal seams.
type SelectionFormatDTO struct {
	Style   StyleDTO       `json:"style"`
	Mixed   []string       `json:"mixed"`
	Borders BorderPatchDTO `json:"borders"`
}

type formatHistogram map[model.Style]int64

type formatSummaryReader struct {
	ctx  context.Context
	wb   *model.Workbook
	sh   *model.Sheet
	work int
	err  error
}

func (r *formatSummaryReader) tick() bool {
	r.work++
	if r.work > 32*command.MaxFillCells {
		r.err = command.ErrFillTooLarge
	}
	if r.work%4096 == 0 && r.err == nil {
		r.err = r.ctx.Err()
	}
	return r.err == nil
}

// Histogram the inherited row/column regions algebraically, then account for
// explicit cells. Even a full worksheet does not enumerate empty coordinates.
func (r *formatSummaryReader) rect(rg model.Range) formatHistogram {
	out := formatHistogram{}
	rows, cols := int64(rg.R1-rg.R0+1), int64(rg.C1-rg.C0+1)
	remaining := rows
	for row, id := range r.sh.Layout.RowStyles {
		if !r.tick() {
			return out
		}
		if row >= rg.R0 && row <= rg.R1 {
			out[r.wb.Styles.Get(id)] += cols
			remaining--
		}
	}
	remainingCols := cols
	for col, id := range r.sh.Layout.ColStyles {
		if !r.tick() {
			return out
		}
		if col >= rg.C0 && col <= rg.C1 {
			out[r.wb.Styles.Get(id)] += remaining
			remainingCols--
		}
	}
	out[r.wb.Styles.Get(0)] += remaining * remainingCols
	r.sh.EachInRange(rg, func(row, col int, c model.Cell) {
		if !r.tick() || c.S == 0 {
			return
		}
		out[r.wb.Styles.Get(r.sh.Layout.StyleAt(row, col))]--
		out[r.wb.Styles.Get(c.S)]++
	})
	return out
}
func addFormatHistogram(to, from formatHistogram, sign int64) {
	for st, n := range from {
		to[st] += sign * n
	}
}
func intersectFormatRange(a, b model.Range) (model.Range, bool) {
	out := model.Range{R0: max(a.R0, b.R0), C0: max(a.C0, b.C0), R1: min(a.R1, b.R1), C1: min(a.C1, b.C1)}
	return out, out.R0 <= out.R1 && out.C0 <= out.C1
}
func formatBorder(st model.Style, side int) model.BorderLine {
	switch side {
	case 0:
		return st.Border.Top
	case 1:
		return st.Border.Bottom
	case 2:
		return st.Border.Left
	default:
		return st.Border.Right
	}
}

// Replace merge fragments with their logical anchor and omit merged interiors.
func (r *formatSummaryReader) visible(rg model.Range, side int) formatHistogram {
	out := r.rect(rg)
	for _, m := range r.sh.Layout.Merges {
		if !r.tick() {
			return out
		}
		hit, ok := intersectFormatRange(rg, m)
		if !ok {
			continue
		}
		addFormatHistogram(out, r.rect(hit), -1)
		include := side < 0
		switch side {
		case 0:
			include = m.R0 >= rg.R0 && m.R0 <= rg.R1
		case 1:
			include = m.R1 >= rg.R0 && m.R1 <= rg.R1
		case 2:
			include = m.C0 >= rg.C0 && m.C0 <= rg.C1
		case 3:
			include = m.C1 >= rg.C0 && m.C1 <= rg.C1
		}
		if include {
			out[r.wb.Styles.Get(r.sh.EffectiveStyle(m.R0, m.C0))]++
		}
	}
	return out
}

func (s *SheetService) SelectionFormat(ctx context.Context, sheet int, ranges []RangeDTO) (*SelectionFormatDTO, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(ranges) > 4096 {
		return nil, command.ErrFillTooLarge
	}
	var out *SelectionFormatDTO
	var err error
	s.at(ctx).read(func(d *document.Document) {
		sh := sheetAt(d, sheet)
		if sh == nil || len(ranges) == 0 {
			return
		}
		wb := d.Workbook()
		reader := formatSummaryReader{ctx: ctx, wb: wb, sh: sh}
		selected := command.ExpandFormatRanges(toModelRanges(ranges), sh.Layout.Merges)
		all := formatHistogram{}
		edgeHist := [6]formatHistogram{}
		for i := range edgeHist {
			edgeHist[i] = formatHistogram{}
		}
		richDelta := map[model.Font]int64{}
		for _, rg := range selected {
			if rg.R0 > rg.R1 || rg.C0 > rg.C1 {
				continue
			}
			addFormatHistogram(all, reader.visible(rg, -1), 1)
			sh.EachInRange(rg, func(row, col int, c model.Cell) {
				if !reader.tick() || c.Rich == nil {
					return
				}
				if m, ok := sh.Layout.MergeAt(row, col); ok && (row != m.R0 || col != m.C0) {
					return
				}
				font := wb.Styles.Get(sh.EffectiveStyle(row, col)).Font
				count := 0
				for _, run := range c.Rich.Runs {
					if !reader.tick() {
						return
					}
					if run.Text != "" {
						richDelta[run.Font.Resolve(font)]++
						count++
					}
				}
				if count > 0 {
					richDelta[font]--
				}
			})
			for side := 0; side < 4; side++ {
				edge := rg
				switch side {
				case 0:
					edge.R1 = edge.R0
				case 1:
					edge.R0 = edge.R1
				case 2:
					edge.C1 = edge.C0
				case 3:
					edge.C0 = edge.C1
				}
				addFormatHistogram(edgeHist[side], reader.visible(edge, side), 1)
			}
			if rg.R1 > rg.R0 {
				upper, lower := rg, rg
				upper.R1--
				lower.R0++
				// Store both faces in one histogram using Top as a canonical slot.
				for st, n := range reader.visible(upper, 1) {
					v := model.Style{Border: model.Borders{Top: st.Border.Bottom}}
					edgeHist[4][v] += n
				}
				for st, n := range reader.visible(lower, 0) {
					v := model.Style{Border: model.Borders{Top: st.Border.Top}}
					edgeHist[4][v] += n
				}
			}
			if rg.C1 > rg.C0 {
				left, right := rg, rg
				left.C1--
				right.C0++
				for st, n := range reader.visible(left, 3) {
					v := model.Style{Border: model.Borders{Top: st.Border.Right}}
					edgeHist[5][v] += n
				}
				for st, n := range reader.visible(right, 2) {
					v := model.Style{Border: model.Borders{Top: st.Border.Left}}
					edgeHist[5][v] += n
				}
			}
			if reader.err != nil {
				err = reader.err
				return
			}
		}
		if err = ctx.Err(); err != nil {
			return
		}
		mixed := map[string]bool{}
		var base model.Style
		found := false
		fonts := map[model.Font]int64{}
		for st, n := range all {
			if n <= 0 {
				continue
			}
			fonts[st.Font] += n
			if !found {
				base = st
				found = true
				continue
			}
			if st.Fill != base.Fill || st.FillSpec != base.FillSpec {
				mixed["fill"] = true
				mixed["fillSpec"] = true
			}
			if st.Border.Diagonal != base.Border.Diagonal {
				mixed["diagonal"] = true
			}
			if st.Border.DiagonalUp != base.Border.DiagonalUp {
				mixed["diagonalUp"] = true
			}
			if st.Border.DiagonalDown != base.Border.DiagonalDown {
				mixed["diagonalDown"] = true
			}
			if st.NumFmt != base.NumFmt {
				mixed["numFmt"] = true
			}
			if st.Align.H != base.Align.H {
				mixed["alignH"] = true
			}
			if st.Align.V != base.Align.V {
				mixed["alignV"] = true
			}
			if st.Align.Wrap != base.Align.Wrap {
				mixed["wrap"] = true
			}
		}
		if !found {
			return
		}
		for font, n := range richDelta {
			fonts[font] += n
		}
		haveFont := false
		for font, n := range fonts {
			if n <= 0 {
				continue
			}
			if !haveFont {
				base.Font = font
				haveFont = true
				continue
			}
			a, b := base.Font, font
			if a.Name != b.Name {
				mixed["fontName"] = true
			}
			if a.Size != b.Size {
				mixed["fontSize"] = true
			}
			if a.Color != b.Color {
				mixed["fontColor"] = true
			}
			if a.Bold != b.Bold {
				mixed["bold"] = true
			}
			if a.Italic != b.Italic {
				mixed["italic"] = true
			}
			if a.Underline != b.Underline {
				mixed["underline"] = true
			}
			if a.Strike != b.Strike {
				mixed["strike"] = true
			}
		}
		out = &SelectionFormatDTO{Style: styleDTO(0, base), Mixed: []string{}}
		fields := []**BorderLineDTO{&out.Borders.Top, &out.Borders.Bottom, &out.Borders.Left, &out.Borders.Right, &out.Borders.InsideH, &out.Borders.InsideV}
		keys := []string{"top", "bottom", "left", "right", "insideH", "insideV"}
		for i, hist := range edgeHist {
			var first model.BorderLine
			have := false
			for st, n := range hist {
				if n <= 0 {
					continue
				}
				side := i
				if i >= 4 {
					side = 0
				}
				line := formatBorder(st, side)
				if !have {
					first = line
					have = true
				} else if line != first {
					mixed[keys[i]] = true
				}
			}
			if !mixed[keys[i]] {
				*fields[i] = &BorderLineDTO{Style: first.Style, Color: first.Color}
			}
		}
		for key := range mixed {
			out.Mixed = append(out.Mixed, key)
		}
		sort.Strings(out.Mixed)
	})
	return out, err
}
