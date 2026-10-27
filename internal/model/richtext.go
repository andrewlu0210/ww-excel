package model

// RichText and its runs are immutable once attached to a cell.
// A pointer keeps ordinary cells comparable and avoids allocating for plain text.
type RichText struct{ Runs []TextRun }
type TextRun struct {
	Text string
	Font *RichFont `json:",omitempty"`
}

// Missing fields inherit the cell font, including explicit false/automatic values.
type RichFont struct {
	Name      *string  `json:",omitempty"`
	Size      *float64 `json:",omitempty"`
	Color     *string  `json:",omitempty"`
	Bold      *bool    `json:",omitempty"`
	Italic    *bool    `json:",omitempty"`
	Underline *bool    `json:",omitempty"`
	Strike    *bool    `json:",omitempty"`
}

func (r *RichFont) Resolve(base Font) Font {
	if r == nil {
		return base
	}
	if r.Name != nil {
		base.Name = *r.Name
	}
	if r.Size != nil {
		base.Size = *r.Size
	}
	if r.Color != nil {
		base.Color = *r.Color
	}
	if r.Bold != nil {
		base.Bold = *r.Bold
	}
	if r.Italic != nil {
		base.Italic = *r.Italic
	}
	if r.Underline != nil {
		base.Underline = *r.Underline
	}
	if r.Strike != nil {
		base.Strike = *r.Strike
	}
	return base
}

func RichFontOf(f Font) *RichFont {
	return &RichFont{Name: &f.Name, Size: &f.Size, Color: &f.Color, Bold: &f.Bold, Italic: &f.Italic, Underline: &f.Underline, Strike: &f.Strike}
}
