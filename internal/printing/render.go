package printing

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // picture decoders
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/signintech/gopdf"

	"ww-excel/internal/model"
)

// CellView is how a cell is shown, as the grid draws it.
type CellView struct {
	Text    string
	Numeric bool // right-aligned by default
	Center  bool // logicals and errors
	Style   model.Style
	Color   string // number-format color (RRGGBB), overrides the font color
}

// Source provides what the renderer cannot compute itself.
type Source interface {
	// View renders a cell (with conditional formats and table styles).
	View(row, col int) (CellView, bool)
	// ObjectImage returns the picture of a floating object (a chart is
	// drawn by the frontend), as PNG or JPEG bytes.
	ObjectImage(o model.Object) []byte
}

// Info fills the header and footer codes.
type Info struct {
	FileName  string
	SheetName string
	Now       time.Time
}

// Font files: a Unicode font for all text (CJK included) and Latin
// bold/italic faces.
var (
	fontRegular = []string{"/System/Library/Fonts/Supplemental/Arial Unicode.ttf", "/Library/Fonts/Arial Unicode.ttf"}
	fontBold    = []string{"/System/Library/Fonts/Supplemental/Arial Bold.ttf"}
	fontItalic  = []string{"/System/Library/Fonts/Supplemental/Arial Italic.ttf"}
)

func findFont(paths []string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

type renderer struct {
	pdf     *gopdf.GoPdf
	sh      *model.Sheet
	plan    *Plan
	src     Source
	info    Info
	bold    bool // a Latin bold face is available
	italic  bool
	current string // font key in use
	size    float64
}

// Render draws the planned pages to PDF.
func Render(sh *model.Sheet, plan *Plan, src Source, info Info) ([]byte, error) {
	regular := findFont(fontRegular)
	if regular == "" {
		return nil, errors.New("找不到列印用的字型（Arial Unicode）")
	}
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: gopdf.Rect{W: plan.PaperW, H: plan.PaperH}, Unit: gopdf.UnitPT})
	pdf.SetInfo(gopdf.PdfInfo{Title: info.SheetName, Creator: "WW Excel", CreationDate: info.Now})
	if err := pdf.AddTTFFont("u", regular); err != nil {
		return nil, fmt.Errorf("字型：%w", err)
	}
	r := &renderer{pdf: pdf, sh: sh, plan: plan, src: src, info: info}
	if p := findFont(fontBold); p != "" && pdf.AddTTFFont("b", p) == nil {
		r.bold = true
	}
	if p := findFont(fontItalic); p != "" && pdf.AddTTFFont("i", p) == nil {
		r.italic = true
	}
	for _, page := range plan.Pages {
		pdf.AddPage()
		r.page(page)
	}
	return pdf.GetBytesPdfReturnErr()
}

func rgb(s string) (uint8, uint8, uint8) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 8 {
		s = s[2:]
	}
	n, _ := strconv.ParseUint(s, 16, 32)
	return uint8(n >> 16), uint8(n >> 8), uint8(n)
}

// geometry of one page: x of each column edge and y of each row edge.
type geometry struct {
	xs, ys []float64
	page   Page
}

func (g geometry) x(col int) float64 { return g.xs[col-g.page.Cols[0]] }
func (g geometry) y(row int) float64 { return g.ys[row-g.page.Rows[0]] }

func (r *renderer) page(pg Page) {
	p := r.plan
	l := &r.sh.Layout
	s := p.Scale
	hw, hh := 0.0, 0.0
	if p.Setup.Headings {
		hw, hh = headingW*s, headingH*s
	}
	// Content size, for centering.
	w, h := hw, hh
	for c := pg.Cols[0]; c <= pg.Cols[1]; c++ {
		w += l.ColWidth(c) * ptPerPx * s
	}
	for rr := pg.Rows[0]; rr <= pg.Rows[1]; rr++ {
		h += l.RowHeight(rr) * ptPerPx * s
	}
	x0, y0 := p.X0, p.Y0
	if p.Setup.CenterH {
		x0 += max(0, (p.W-w)/2)
	}
	if p.Setup.CenterV {
		y0 += max(0, (p.H-h)/2)
	}
	g := geometry{page: pg}
	x := x0 + hw
	for c := pg.Cols[0]; c <= pg.Cols[1]+1; c++ {
		g.xs = append(g.xs, x)
		x += l.ColWidth(c) * ptPerPx * s
	}
	y := y0 + hh
	for rr := pg.Rows[0]; rr <= pg.Rows[1]+1; rr++ {
		g.ys = append(g.ys, y)
		y += l.RowHeight(rr) * ptPerPx * s
	}
	right, bottom := g.xs[len(g.xs)-1], g.ys[len(g.ys)-1]

	// Clip to the page's cells.
	r.pdf.SaveGraphicsState()
	r.clip(x0+hw, y0+hh, right, bottom)
	merges := r.merges(pg)
	covered := map[[2]int]bool{}
	for _, m := range merges {
		for rr := max(m.R0, pg.Rows[0]); rr <= min(m.R1, pg.Rows[1]); rr++ {
			for c := max(m.C0, pg.Cols[0]); c <= min(m.C1, pg.Cols[1]); c++ {
				covered[[2]int{rr, c}] = true
			}
		}
	}
	views := map[[2]int]CellView{}
	for rr := pg.Rows[0]; rr <= pg.Rows[1]; rr++ {
		for c := pg.Cols[0]; c <= pg.Cols[1]; c++ {
			if v, ok := r.src.View(rr, c); ok {
				views[[2]int{rr, c}] = v
			}
		}
	}
	// Fills, then gridlines, borders and text.
	for key, v := range views {
		if v.Style.Fill == "" || covered[key] {
			continue
		}
		r.fill(v.Style.Fill, g.x(key[1]), g.y(key[0]), g.x(key[1]+1), g.y(key[0]+1))
	}
	for _, m := range merges {
		if v, ok := r.src.View(m.R0, m.C0); ok && v.Style.Fill != "" {
			x0, y0, x1, y1 := r.mergeRect(g, m)
			r.fill(v.Style.Fill, x0, y0, x1, y1)
		}
	}
	if p.Setup.Gridlines {
		r.gridlines(g, covered)
	}
	for key, v := range views {
		if covered[key] {
			continue
		}
		r.borders(v.Style, g.x(key[1]), g.y(key[0]), g.x(key[1]+1), g.y(key[0]+1))
	}
	for key, v := range views {
		if covered[key] || v.Text == "" {
			continue
		}
		x0, y0, x1, y1 := g.x(key[1]), g.y(key[0]), g.x(key[1]+1), g.y(key[0]+1)
		// Left-aligned text runs into empty cells to the right.
		clipR := x1
		if !v.Numeric && !v.Style.Align.Wrap && (v.Style.Align.H == "" || v.Style.Align.H == "left" || v.Style.Align.H == "general") {
			for c := key[1] + 1; c <= pg.Cols[1]; c++ {
				if n, ok := views[[2]int{key[0], c}]; (ok && n.Text != "") || covered[[2]int{key[0], c}] {
					break
				}
				clipR = g.x(c + 1)
			}
		}
		r.text(v, x0, y0, x1, y1, clipR)
	}
	for _, m := range merges {
		if v, ok := r.src.View(m.R0, m.C0); ok {
			x0, y0, x1, y1 := r.mergeRect(g, m)
			r.borders(v.Style, x0, y0, x1, y1)
			if v.Text != "" {
				r.text(v, x0, y0, x1, y1, x1)
			}
		}
	}
	r.objects(g, x0+hw, y0+hh)
	r.pdf.RestoreGraphicsState()

	if p.Setup.Headings {
		r.headings(g, x0, y0, hw, hh)
	}
	r.headerFooter(pg)
}

func (r *renderer) clip(x0, y0, x1, y1 float64) {
	r.pdf.ClipPolygon([]gopdf.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}})
}

func (r *renderer) fill(color string, x0, y0, x1, y1 float64) {
	cr, cg, cb := rgb(color)
	r.pdf.SetFillColor(cr, cg, cb)
	r.pdf.RectFromUpperLeftWithStyle(x0, y0, x1-x0, y1-y0, "F")
}

func (r *renderer) merges(pg Page) []model.Range {
	var out []model.Range
	view := model.Range{R0: pg.Rows[0], C0: pg.Cols[0], R1: pg.Rows[1], C1: pg.Cols[1]}
	for _, m := range r.sh.Layout.Merges {
		if m.Overlaps(view) {
			out = append(out, m)
		}
	}
	return out
}

// mergeRect is a merged area's rectangle, extended past the page edge
// when the merge continues on another page (clipped anyway).
func (r *renderer) mergeRect(g geometry, m model.Range) (float64, float64, float64, float64) {
	l := &r.sh.Layout
	s := r.plan.Scale
	edgeX := func(col int) float64 {
		if col < g.page.Cols[0] {
			x := g.x(g.page.Cols[0])
			for c := col; c < g.page.Cols[0]; c++ {
				x -= l.ColWidth(c) * ptPerPx * s
			}
			return x
		}
		if col > g.page.Cols[1]+1 {
			x := g.x(g.page.Cols[1] + 1)
			for c := g.page.Cols[1] + 1; c < col; c++ {
				x += l.ColWidth(c) * ptPerPx * s
			}
			return x
		}
		return g.x(col)
	}
	edgeY := func(row int) float64 {
		if row < g.page.Rows[0] {
			y := g.y(g.page.Rows[0])
			for rr := row; rr < g.page.Rows[0]; rr++ {
				y -= l.RowHeight(rr) * ptPerPx * s
			}
			return y
		}
		if row > g.page.Rows[1]+1 {
			y := g.y(g.page.Rows[1] + 1)
			for rr := g.page.Rows[1] + 1; rr < row; rr++ {
				y += l.RowHeight(rr) * ptPerPx * s
			}
			return y
		}
		return g.y(row)
	}
	return edgeX(m.C0), edgeY(m.R0), edgeX(m.C1 + 1), edgeY(m.R1 + 1)
}

func (r *renderer) gridlines(g geometry, covered map[[2]int]bool) {
	r.pdf.SetStrokeColor(200, 200, 200)
	r.pdf.SetLineWidth(0.25)
	x0, x1 := g.xs[0], g.xs[len(g.xs)-1]
	y0, y1 := g.ys[0], g.ys[len(g.ys)-1]
	for _, y := range g.ys {
		r.pdf.Line(x0, y, x1, y)
	}
	for _, x := range g.xs {
		r.pdf.Line(x, y0, x, y1)
	}
}

var borderWidth = map[string]float64{"medium": 1.25, "thick": 2, "double": 0.5}

func (r *renderer) borders(st model.Style, x0, y0, x1, y1 float64) {
	b := st.Border
	line := func(l model.BorderLine, ax, ay, bx, by float64) {
		if l.Style == "" {
			return
		}
		cr, cg, cb := uint8(0), uint8(0), uint8(0)
		if l.Color != "" {
			cr, cg, cb = rgb(l.Color)
		}
		r.pdf.SetStrokeColor(cr, cg, cb)
		w := borderWidth[l.Style]
		if w == 0 {
			w = 0.5
		}
		r.pdf.SetLineWidth(w)
		switch {
		case strings.Contains(strings.ToLower(l.Style), "dash"):
			r.pdf.SetLineType("dashed")
		case l.Style == "dotted" || l.Style == "hair":
			r.pdf.SetLineType("dotted")
		default:
			r.pdf.SetLineType("solid")
		}
		r.pdf.Line(ax, ay, bx, by)
		if l.Style == "double" {
			if ay == by {
				r.pdf.Line(ax, ay+1.5, bx, by+1.5)
			} else {
				r.pdf.Line(ax+1.5, ay, bx+1.5, by)
			}
		}
		r.pdf.SetLineType("solid")
	}
	line(b.Top, x0, y0, x1, y0)
	line(b.Bottom, x0, y1, x1, y1)
	line(b.Left, x0, y0, x0, y1)
	line(b.Right, x1, y0, x1, y1)
}

// latin1 reports whether a Latin face can show the text.
func latin1(s string) bool {
	for _, r := range s {
		if r > 0xFF {
			return false
		}
	}
	return true
}

func (r *renderer) setFont(st model.Style, text string, scale float64) {
	size := st.Font.Size
	if size <= 0 {
		size = 11
	}
	size *= scale
	key := "u"
	if latin1(text) {
		if st.Font.Bold && r.bold {
			key = "b"
		} else if st.Font.Italic && r.italic {
			key = "i"
		}
	}
	if key != r.current || size != r.size {
		_ = r.pdf.SetFont(key, "", size)
		r.current, r.size = key, size
	}
}

func (r *renderer) text(v CellView, x0, y0, x1, y1, clipR float64) {
	st := v.Style
	s := r.plan.Scale
	r.setFont(st, v.Text, s)
	size := r.size
	color := st.Font.Color
	if v.Color != "" {
		color = v.Color
	}
	cr, cg, cb := uint8(0), uint8(0), uint8(0)
	if color != "" {
		cr, cg, cb = rgb(color)
	}
	r.pdf.SetTextColor(cr, cg, cb)

	pad := 2.25 * s
	align := st.Align.H
	if align == "" || align == "general" {
		switch {
		case v.Numeric:
			align = "right"
		case v.Center:
			align = "center"
		default:
			align = "left"
		}
	}
	var lines []string
	if st.Align.Wrap {
		lines = r.wrap(v.Text, x1-x0-2*pad)
	} else {
		lines = []string{strings.ReplaceAll(v.Text, "\n", " ")}
	}
	lineH := size * 1.2
	total := float64(len(lines)) * lineH
	var y float64
	switch st.Align.V {
	case "top":
		y = y0 + 1
	case "center":
		y = y0 + (y1-y0-total)/2
	default:
		y = y1 - total - 1
	}
	r.pdf.SaveGraphicsState()
	left := x0
	if align == "right" || align == "center" {
		clipR = x1
	}
	r.clip(left, y0, clipR, y1)
	for _, line := range lines {
		tw, _ := r.pdf.MeasureTextWidth(line)
		x := x0 + pad + float64(st.Align.Indent)*9*ptPerPx*s
		switch align {
		case "right":
			x = x1 - pad - tw
		case "center", "centerContinuous":
			x = x0 + (x1-x0-tw)/2
		}
		// Text is placed by its baseline.
		base := y + size*0.92
		r.pdf.SetXY(x, base)
		_ = r.pdf.Text(line)
		if st.Font.Bold && r.current == "u" {
			// No bold face for this text: overprint slightly offset.
			r.pdf.SetXY(x+size*0.04, base)
			_ = r.pdf.Text(line)
		}
		if st.Font.Underline {
			r.pdf.SetStrokeColor(cr, cg, cb)
			r.pdf.SetLineWidth(size / 16)
			r.pdf.Line(x, y+size*1.05, x+tw, y+size*1.05)
		}
		if st.Font.Strike {
			r.pdf.SetStrokeColor(cr, cg, cb)
			r.pdf.SetLineWidth(size / 16)
			r.pdf.Line(x, y+size*0.6, x+tw, y+size*0.6)
		}
		y += lineH
	}
	r.pdf.RestoreGraphicsState()
	r.current = "" // the font state was restored
}

// wrap breaks text into lines that fit width, at spaces or between CJK
// characters.
func (r *renderer) wrap(text string, width float64) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range splitWords(para) {
			next := line + word
			if w, _ := r.pdf.MeasureTextWidth(next); w > width && line != "" {
				out = append(out, strings.TrimRight(line, " "))
				next = strings.TrimLeft(word, " ")
			}
			line = next
		}
		out = append(out, line)
	}
	return out
}

// splitWords splits at spaces (kept with the word) and around wide
// characters, which may break anywhere.
func splitWords(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		switch {
		case r == ' ':
			cur += " "
			out = append(out, cur)
			cur = ""
		case utf8.RuneLen(r) > 2:
			if cur != "" {
				out = append(out, cur)
			}
			out = append(out, string(r))
			cur = ""
		default:
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// objects draws the charts and pictures over this page.
func (r *renderer) objects(g geometry, left, top float64) {
	l := &r.sh.Layout
	s := r.plan.Scale
	for _, o := range l.Objects {
		row, col := o.Anchor.Row, o.Anchor.Col
		endR, endC := ObjectEnd(l, o)
		if endR < g.page.Rows[0] || row > g.page.Rows[1] || endC < g.page.Cols[0] || col > g.page.Cols[1] {
			continue
		}
		// Top-left relative to the page origin, also when the anchor is
		// on an earlier page.
		x := left
		if col >= g.page.Cols[0] {
			x = g.x(col)
		} else {
			for c := col; c < g.page.Cols[0]; c++ {
				x -= l.ColWidth(c) * ptPerPx * s
			}
		}
		y := top
		if row >= g.page.Rows[0] {
			y = g.y(row)
		} else {
			for rr := row; rr < g.page.Rows[0]; rr++ {
				y -= l.RowHeight(rr) * ptPerPx * s
			}
		}
		x += o.Anchor.DX * ptPerPx * s
		y += o.Anchor.DY * ptPerPx * s
		data := r.src.ObjectImage(o)
		if data == nil {
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		_ = r.pdf.ImageFrom(img, x, y, &gopdf.Rect{W: o.W * ptPerPx * s, H: o.H * ptPerPx * s})
	}
}

func (r *renderer) headings(g geometry, x0, y0, hw, hh float64) {
	st := model.Style{Font: model.Font{Size: 9}}
	r.pdf.SetStrokeColor(160, 160, 160)
	r.pdf.SetLineWidth(0.25)
	for c := g.page.Cols[0]; c <= g.page.Cols[1]; c++ {
		a, b := g.x(c), g.x(c+1)
		if b <= a {
			continue
		}
		r.fill("F2F2F2", a, y0, b, y0+hh)
		r.pdf.RectFromUpperLeftWithStyle(a, y0, b-a, hh, "D")
		r.text(CellView{Text: colName(c), Center: true, Style: withAlign(st, "center")}, a, y0, b, y0+hh, b)
	}
	for rr := g.page.Rows[0]; rr <= g.page.Rows[1]; rr++ {
		a, b := g.y(rr), g.y(rr+1)
		if b <= a {
			continue
		}
		r.fill("F2F2F2", x0, a, x0+hw, b)
		r.pdf.RectFromUpperLeftWithStyle(x0, a, hw, b-a, "D")
		r.text(CellView{Text: strconv.Itoa(rr + 1), Center: true, Style: withAlign(st, "center")}, x0, a, x0+hw, b, x0+hw)
	}
}

func withAlign(st model.Style, h string) model.Style {
	st.Align.H, st.Align.V = h, "center"
	return st
}

func colName(c int) string {
	s := ""
	for n := c + 1; n > 0; n = (n - 1) / 26 {
		s = string(rune('A'+(n-1)%26)) + s
	}
	return s
}

// headerFooter draws the page header and footer (center section).
func (r *renderer) headerFooter(pg Page) {
	ps := r.plan.Setup
	expand := func(s string) string {
		repl := strings.NewReplacer(
			"&P", strconv.Itoa(pg.Number),
			"&N", strconv.Itoa(len(r.plan.Pages)),
			"&D", r.info.Now.Format("2006/1/2"),
			"&T", r.info.Now.Format("15:04"),
			"&F", r.info.FileName,
			"&A", r.info.SheetName,
			"&&", "&",
		)
		return repl.Replace(s)
	}
	st := model.Style{Font: model.Font{Size: 10}}
	draw := func(text string, y float64) {
		if text == "" {
			return
		}
		text = expand(text)
		r.setFont(st, text, 1)
		r.pdf.SetTextColor(0, 0, 0)
		w, _ := r.pdf.MeasureTextWidth(text)
		r.pdf.SetXY((r.plan.PaperW-w)/2, y)
		_ = r.pdf.Text(text)
	}
	// y is the text baseline.
	draw(ps.HeaderText, ps.Header*ptPerInch+10)
	draw(ps.FooterText, r.plan.PaperH-ps.Footer*ptPerInch)
}
