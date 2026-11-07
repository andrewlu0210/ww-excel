package service

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"ww-excel/internal/command"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

type FontDTO struct {
	Name      string  `json:"name"`
	Size      float64 `json:"size"`
	Bold      bool    `json:"bold,omitempty"`
	Italic    bool    `json:"italic,omitempty"`
	Underline bool    `json:"underline,omitempty"`
	Strike    bool    `json:"strike,omitempty"`
	Color     string  `json:"color,omitempty"`
}

type BorderLineDTO struct {
	Style string `json:"style,omitempty"`
	Color string `json:"color,omitempty"`
}

type StyleDTO struct {
	ID           uint32         `json:"id"`
	Font         FontDTO        `json:"font"`
	Fill         string         `json:"fill,omitempty"`
	FillSpec     model.FillSpec `json:"fillSpec,omitempty"`
	Diagonal     BorderLineDTO  `json:"diagonal,omitempty"`
	DiagonalUp   bool           `json:"diagonalUp,omitempty"`
	DiagonalDown bool           `json:"diagonalDown,omitempty"`
	Left         BorderLineDTO  `json:"left"`
	Right        BorderLineDTO  `json:"right"`
	Top          BorderLineDTO  `json:"top"`
	Bottom       BorderLineDTO  `json:"bottom"`
	AlignH       string         `json:"alignH,omitempty"`
	AlignV       string         `json:"alignV,omitempty"`
	Wrap         bool           `json:"wrap,omitempty"`
	Indent       int            `json:"indent,omitempty"`
	NumFmt       string         `json:"numFmt,omitempty"`
}

func styleDTO(id uint32, st model.Style) StyleDTO {
	line := func(l model.BorderLine) BorderLineDTO { return BorderLineDTO{Style: l.Style, Color: l.Color} }
	f := st.Font
	return StyleDTO{
		ID:   id,
		Font: FontDTO{Name: f.Name, Size: f.Size, Bold: f.Bold, Italic: f.Italic, Underline: f.Underline, Strike: f.Strike, Color: f.Color},
		Fill: st.Fill, FillSpec: st.FillSpec, Diagonal: line(st.Border.Diagonal), DiagonalUp: st.Border.DiagonalUp, DiagonalDown: st.Border.DiagonalDown,
		Left:   line(st.Border.Left),
		Right:  line(st.Border.Right),
		Top:    line(st.Border.Top),
		Bottom: line(st.Border.Bottom),
		AlignH: st.Align.H,
		AlignV: st.Align.V,
		Wrap:   st.Align.Wrap,
		Indent: st.Align.Indent,
		NumFmt: st.NumFmt,
	}
}

// stylesSince lists styles with ID >= from.
func stylesSince(wb *model.Workbook, from int) []StyleDTO {
	list := wb.StyleTable().Since(from)
	out := make([]StyleDTO, len(list))
	for i, st := range list {
		out[i] = styleDTO(uint32(from+i), st)
	}
	return out
}

// BorderPatchDTO distinguishes omitted edges from explicit clearing ({}).
type BorderPatchDTO struct {
	Top     *BorderLineDTO `json:"top,omitempty"`
	Bottom  *BorderLineDTO `json:"bottom,omitempty"`
	Left    *BorderLineDTO `json:"left,omitempty"`
	Right   *BorderLineDTO `json:"right,omitempty"`
	InsideH *BorderLineDTO `json:"insideH,omitempty"`
	InsideV *BorderLineDTO `json:"insideV,omitempty"`
}

var borderColorPattern = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

func (p *BorderPatchDTO) model() (command.BorderPatch, error) {
	out := command.BorderPatch{}
	for _, pair := range []struct {
		in  *BorderLineDTO
		out **model.BorderLine
	}{
		{p.Top, &out.Top}, {p.Bottom, &out.Bottom}, {p.Left, &out.Left}, {p.Right, &out.Right}, {p.InsideH, &out.InsideH}, {p.InsideV, &out.InsideV},
	} {
		if pair.in == nil {
			continue
		}
		line := pair.in
		switch line.Style {
		case "", "thin", "medium", "thick", "dashed", "dotted", "double", "hair", "mediumDashed", "dashDot", "mediumDashDot", "dashDotDot", "mediumDashDotDot", "slantDashDot":
		default:
			return out, errors.New("不支援的框線樣式")
		}
		if line.Color != "" && !borderColorPattern.MatchString(line.Color) {
			return out, errors.New("框線色彩必須是六位十六進位色碼")
		}
		next := model.BorderLine{Style: line.Style, Color: strings.ToUpper(line.Color)}
		if next.Style == "" {
			next.Color = ""
		}
		*pair.out = &next
	}
	return out, nil
}

// presetBorderPatch keeps toolbar operations consistent with dialog edits.
func presetBorderPatch(preset, color string) *BorderPatchDTO {
	thin := &BorderLineDTO{Style: "thin", Color: color}
	empty := &BorderLineDTO{}
	switch preset {
	case "none":
		return &BorderPatchDTO{Top: empty, Bottom: empty, Left: empty, Right: empty, InsideH: empty, InsideV: empty}
	case "all":
		return &BorderPatchDTO{Top: thin, Bottom: thin, Left: thin, Right: thin, InsideH: thin, InsideV: thin}
	case "outside", "thickOutside":
		if preset == "thickOutside" {
			thin.Style = "medium"
		}
		return &BorderPatchDTO{Top: thin, Bottom: thin, Left: thin, Right: thin}
	case "top":
		return &BorderPatchDTO{Top: thin}
	case "left":
		return &BorderPatchDTO{Left: thin}
	case "right":
		return &BorderPatchDTO{Right: thin}
	case "bottom", "thickBottom", "doubleBottom":
		if preset == "thickBottom" {
			thin.Style = "medium"
		}
		if preset == "doubleBottom" {
			thin.Style = "double"
		}
		return &BorderPatchDTO{Bottom: thin}
	}
	return nil
}

// FormatPatch is a formatting change from the toolbar; nil fields are
// left as they are.
type FormatPatch struct {
	RestyleBorders *string         `json:"restyleBorders,omitempty"`
	RecolorBorders *string         `json:"recolorBorders,omitempty"`
	FillSpec       *model.FillSpec `json:"fillSpec,omitempty"`
	Diagonal       *BorderLineDTO  `json:"diagonal,omitempty"`
	DiagonalUp     *bool           `json:"diagonalUp,omitempty"`
	DiagonalDown   *bool           `json:"diagonalDown,omitempty"`
	AutoRows       []SizeDTO       `json:"autoRows,omitempty"`
	Borders        *BorderPatchDTO `json:"borders,omitempty"`
	FontName       *string         `json:"fontName,omitempty"`
	FontSize       *float64        `json:"fontSize,omitempty"`
	Bold           *bool           `json:"bold,omitempty"`
	Italic         *bool           `json:"italic,omitempty"`
	Underline      *bool           `json:"underline,omitempty"`
	Strike         *bool           `json:"strike,omitempty"`
	FontColor      *string         `json:"fontColor,omitempty"`
	Fill           *string         `json:"fill,omitempty"`
	AlignH         *string         `json:"alignH,omitempty"`
	AlignV         *string         `json:"alignV,omitempty"`
	Wrap           *bool           `json:"wrap,omitempty"`
	NumFmt         *string         `json:"numFmt,omitempty"`
	// Decimals adds (+) or removes (-) decimal places.
	Decimals int `json:"decimals,omitempty"`
	// Border applies a preset: bottom, top, left, right, all, outside,
	// thickOutside, thickBottom, doubleBottom, none.
	Border string `json:"border,omitempty"`
	// BorderColor for the preset; "" is automatic.
	BorderColor string `json:"borderColor,omitempty"`
	// Label names the change in the Undo menu.
	Label string `json:"label,omitempty"`
}

func (p FormatPatch) apply(st model.Style, _ command.At) model.Style {
	set := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	if p.FontName != nil {
		st.Font.Name = *p.FontName
	}
	if p.FontSize != nil && *p.FontSize > 0 {
		st.Font.Size = *p.FontSize
	}
	set(&st.Font.Bold, p.Bold)
	set(&st.Font.Italic, p.Italic)
	set(&st.Font.Underline, p.Underline)
	set(&st.Font.Strike, p.Strike)
	if p.FontColor != nil {
		st.Font.Color = *p.FontColor
	}
	if p.Fill != nil {
		st.Fill = *p.Fill
		st.FillSpec = model.FillSpec{}
	}
	if p.FillSpec != nil {
		st.FillSpec = *p.FillSpec
		st.Fill = ""
	}
	if p.Diagonal != nil {
		st.Border.Diagonal = model.BorderLine{Style: p.Diagonal.Style, Color: strings.ToUpper(p.Diagonal.Color)}
	}
	set(&st.Border.DiagonalUp, p.DiagonalUp)
	set(&st.Border.DiagonalDown, p.DiagonalDown)
	if st.Border.Diagonal.Style == "" {
		st.Border.DiagonalUp = false
		st.Border.DiagonalDown = false
	}
	if !st.Border.DiagonalUp && !st.Border.DiagonalDown {
		st.Border.Diagonal = model.BorderLine{}
	}
	if p.RecolorBorders != nil || p.RestyleBorders != nil {
		for _, line := range []*model.BorderLine{&st.Border.Top, &st.Border.Bottom, &st.Border.Left, &st.Border.Right, &st.Border.Diagonal} {
			if line.Style != "" {
				if p.RecolorBorders != nil {
					line.Color = *p.RecolorBorders
				}
				if p.RestyleBorders != nil {
					line.Style = *p.RestyleBorders
				}
			}
		}
	}
	if p.AlignH != nil {
		st.Align.H = *p.AlignH
	}
	if p.AlignV != nil {
		st.Align.V = *p.AlignV
	}
	set(&st.Align.Wrap, p.Wrap)
	if p.NumFmt != nil {
		st.NumFmt = *p.NumFmt
	}
	if p.Decimals != 0 {
		st.NumFmt = numfmt.AdjustDecimals(st.NumFmt, p.Decimals)
	}
	return st
}

// display renders a cell with its number format; color is the format's
// section color (e.g. [Red]) or "".
func display(c model.Cell, st model.Style) (text, color string) {
	switch c.Kind {
	case model.KindNumber:
		return numfmt.Render(c.Num, st.NumFmt)
	case model.KindString:
		return numfmt.RenderText(c.Str, st.NumFmt)
	}
	return c.Display(), ""
}

type SizeDTO struct {
	I      int     `json:"i"`
	Px     float64 `json:"px"`
	Hidden bool    `json:"hidden,omitempty"`
}

type StyleRefDTO struct {
	I int    `json:"i"`
	S uint32 `json:"s"`
}

// LayoutDTO is a sheet's geometry, whole-row/column styles and merges.
type LayoutDTO struct {
	PivotButtons  []model.PivotButton `json:"pivotButtons,omitempty"`
	PivotHeaders  []RangeDTO          `json:"pivotHeaders,omitempty"`
	HideGridLines bool                `json:"hideGridLines,omitempty"`
	DefaultRow    float64             `json:"defaultRow"`
	DefaultCol    float64             `json:"defaultCol"`
	Rows          []SizeDTO           `json:"rows"`
	Cols          []SizeDTO           `json:"cols"`
	RowStyles     []StyleRefDTO       `json:"rowStyles"`
	ColStyles     []StyleRefDTO       `json:"colStyles"`
	Merges        []RangeDTO          `json:"merges"`
	FreezeRows    int                 `json:"freezeRows"`
	FreezeCols    int                 `json:"freezeCols"`
	// Filter is the AutoFilter range, or nil; FilteredCols are the
	// columns with criteria (their buttons show a funnel).
	Filter       *RangeDTO `json:"filter"`
	FilteredCols []int     `json:"filteredCols"`
	// Notes (red triangles) and tables.
	Notes   []NoteDTO   `json:"notes"`
	Tables  []TableDTO  `json:"tables"`
	Objects []ObjectDTO `json:"objects"`
	// Styles the caller does not know yet (IDs >= the known count).
	Styles []StyleDTO `json:"styles"`
}

func layoutDTO(wb *model.Workbook, l model.Layout, knownStyles int) LayoutDTO {
	out := LayoutDTO{
		HideGridLines: l.HideGridLines, DefaultRow: l.DefaultRow, DefaultCol: l.DefaultCol,
		Rows: []SizeDTO{}, Cols: []SizeDTO{}, RowStyles: []StyleRefDTO{}, ColStyles: []StyleRefDTO{}, Merges: []RangeDTO{},
		Styles:     stylesSince(wb, knownStyles),
		FreezeRows: l.FreezeRows, FreezeCols: l.FreezeCols, FilteredCols: []int{},
		Notes: []NoteDTO{}, Tables: []TableDTO{}, Objects: []ObjectDTO{},
	}
	for _, o := range l.Objects {
		out.Objects = append(out.Objects, objectDTO(o))
	}
	for pos, n := range l.Notes {
		out.Notes = append(out.Notes, NoteDTO{Row: pos[0], Col: pos[1], Author: n.Author, Text: n.Text})
	}
	sort.Slice(out.Notes, func(i, j int) bool {
		a, b := out.Notes[i], out.Notes[j]
		return a.Row < b.Row || a.Row == b.Row && a.Col < b.Col
	})
	for _, p := range l.Pivots {
		out.PivotButtons = append(out.PivotButtons, p.Buttons()...)
	}
	for _, h := range l.PivotHeaders {
		out.PivotHeaders = append(out.PivotHeaders, rangeDTO(h))
	}
	for _, t := range l.Tables {
		out.Tables = append(out.Tables, tableDTO(t))
	}
	if f := l.Filter; f != nil {
		out.Filter = &RangeDTO{R0: f.Range.R0, C0: f.Range.C0, R1: f.Range.R1, C1: f.Range.C1}
		for c := range f.Cols {
			out.FilteredCols = append(out.FilteredCols, c)
		}
		sort.Ints(out.FilteredCols)
	}
	for i, s := range l.Rows {
		out.Rows = append(out.Rows, SizeDTO{I: i, Px: s.Px, Hidden: s.Hidden})
	}
	for i, s := range l.Cols {
		out.Cols = append(out.Cols, SizeDTO{I: i, Px: s.Px, Hidden: s.Hidden})
	}
	for i, s := range l.RowStyles {
		out.RowStyles = append(out.RowStyles, StyleRefDTO{I: i, S: s})
	}
	for i, s := range l.ColStyles {
		out.ColStyles = append(out.ColStyles, StyleRefDTO{I: i, S: s})
	}
	for _, m := range l.Merges {
		out.Merges = append(out.Merges, RangeDTO{R0: m.R0, C0: m.C0, R1: m.R1, C1: m.C1})
	}
	return out
}
