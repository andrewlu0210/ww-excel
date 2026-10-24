package service

import (
	"sort"
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
	ID     uint32        `json:"id"`
	Font   FontDTO       `json:"font"`
	Fill   string        `json:"fill,omitempty"`
	Left   BorderLineDTO `json:"left"`
	Right  BorderLineDTO `json:"right"`
	Top    BorderLineDTO `json:"top"`
	Bottom BorderLineDTO `json:"bottom"`
	AlignH string        `json:"alignH,omitempty"`
	AlignV string        `json:"alignV,omitempty"`
	Wrap   bool          `json:"wrap,omitempty"`
	Indent int           `json:"indent,omitempty"`
	NumFmt string        `json:"numFmt,omitempty"`
}

func styleDTO(id uint32, st model.Style) StyleDTO {
	line := func(l model.BorderLine) BorderLineDTO { return BorderLineDTO{Style: l.Style, Color: l.Color} }
	f := st.Font
	return StyleDTO{
		ID:     id,
		Font:   FontDTO{Name: f.Name, Size: f.Size, Bold: f.Bold, Italic: f.Italic, Underline: f.Underline, Strike: f.Strike, Color: f.Color},
		Fill:   st.Fill,
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

// FormatPatch is a formatting change from the toolbar; nil fields are
// left as they are.
type FormatPatch struct {
	FontName  *string  `json:"fontName,omitempty"`
	FontSize  *float64 `json:"fontSize,omitempty"`
	Bold      *bool    `json:"bold,omitempty"`
	Italic    *bool    `json:"italic,omitempty"`
	Underline *bool    `json:"underline,omitempty"`
	Strike    *bool    `json:"strike,omitempty"`
	FontColor *string  `json:"fontColor,omitempty"`
	Fill      *string  `json:"fill,omitempty"`
	AlignH    *string  `json:"alignH,omitempty"`
	AlignV    *string  `json:"alignV,omitempty"`
	Wrap      *bool    `json:"wrap,omitempty"`
	NumFmt    *string  `json:"numFmt,omitempty"`
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

func (p FormatPatch) apply(st model.Style, at command.At) model.Style {
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
	if p.Border != "" {
		st.Border = borderPreset(st.Border, p.Border, p.BorderColor, at)
	}
	return st
}

// borderPreset applies an Excel border button to one position. Edge
// presets only touch cells on that edge of the range.
func borderPreset(b model.Borders, preset, color string, at command.At) model.Borders {
	thin := model.BorderLine{Style: "thin", Color: color}
	r := at.Range
	// Whole rows/columns: Row or Col is -1, treat as on every edge
	// along that axis.
	top := at.Row == r.R0 || at.Row < 0
	bottom := at.Row == r.R1 || at.Row < 0
	left := at.Col == r.C0 || at.Col < 0
	right := at.Col == r.C1 || at.Col < 0
	switch preset {
	case "none":
		return model.Borders{}
	case "all":
		return model.Borders{Left: thin, Right: thin, Top: thin, Bottom: thin}
	case "outside", "thickOutside":
		line := thin
		if preset == "thickOutside" {
			line.Style = "medium"
		}
		if top {
			b.Top = line
		}
		if bottom {
			b.Bottom = line
		}
		if left {
			b.Left = line
		}
		if right {
			b.Right = line
		}
	case "bottom", "thickBottom", "doubleBottom":
		line := thin
		switch preset {
		case "thickBottom":
			line.Style = "medium"
		case "doubleBottom":
			line.Style = "double"
		}
		if bottom {
			b.Bottom = line
		}
	case "top":
		if top {
			b.Top = thin
		}
	case "left":
		if left {
			b.Left = thin
		}
	case "right":
		if right {
			b.Right = thin
		}
	}
	return b
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
	DefaultRow float64       `json:"defaultRow"`
	DefaultCol float64       `json:"defaultCol"`
	Rows       []SizeDTO     `json:"rows"`
	Cols       []SizeDTO     `json:"cols"`
	RowStyles  []StyleRefDTO `json:"rowStyles"`
	ColStyles  []StyleRefDTO `json:"colStyles"`
	Merges     []RangeDTO    `json:"merges"`
	FreezeRows int           `json:"freezeRows"`
	FreezeCols int           `json:"freezeCols"`
	// Filter is the AutoFilter range, or nil; FilteredCols are the
	// columns with criteria (their buttons show a funnel).
	Filter       *RangeDTO `json:"filter"`
	FilteredCols []int     `json:"filteredCols"`
	// Styles the caller does not know yet (IDs >= the known count).
	Styles []StyleDTO `json:"styles"`
}

func layoutDTO(wb *model.Workbook, l model.Layout, knownStyles int) LayoutDTO {
	out := LayoutDTO{
		DefaultRow: l.DefaultRow, DefaultCol: l.DefaultCol,
		Rows: []SizeDTO{}, Cols: []SizeDTO{}, RowStyles: []StyleRefDTO{}, ColStyles: []StyleRefDTO{}, Merges: []RangeDTO{},
		Styles:     stylesSince(wb, knownStyles),
		FreezeRows: l.FreezeRows, FreezeCols: l.FreezeCols, FilteredCols: []int{},
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
