package model

import "strings"

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

// PreserveRichText carries formatting through a plain-text input commit.
// Explicit rich text and conversions to non-text values take precedence.
func PreserveRichText(old, next Cell) Cell {
	if old.Rich == nil || old.Kind != KindString || old.F != nil ||
		next.Rich != nil || next.Kind != KindString || next.F != nil {
		return next
	}
	if old.Str == next.Str {
		next.Rich = old.Rich
		return next
	}
	var text strings.Builder
	var fonts []*RichFont
	for _, run := range old.Rich.Runs {
		text.WriteString(run.Text)
		for range []rune(run.Text) {
			fonts = append(fonts, run.Font)
		}
	}
	if text.String() != old.Str || len(fonts) == 0 {
		return next // Do not apply stale run offsets to a different value.
	}
	a, b := []rune(old.Str), []rune(next.Str)
	matches := richTextMatches(a, b)
	rich := &RichText{}
	var pending strings.Builder
	var pendingFont *RichFont
	flush := func() {
		if pending.Len() > 0 {
			rich.Runs = append(rich.Runs, TextRun{Text: pending.String(), Font: pendingFont})
			pending.Reset()
		}
	}
	appendRun := func(text string, font *RichFont) {
		if text == "" {
			return
		}
		if !sameRichFont(pendingFont, font) {
			flush()
		}
		pendingFont = font
		pending.WriteString(text)
	}
	oi, ni := 0, 0
	for _, match := range append(matches, [2]int{len(a), len(b)}) {
		if ni < match[1] {
			// Replacements inherit the first removed character. Insertions
			// inherit the left character, or the first font at the start.
			from := oi
			if oi == match[0] && oi > 0 {
				from--
			}
			appendRun(string(b[ni:match[1]]), fonts[min(from, len(fonts)-1)])
		}
		if match[0] < len(a) {
			appendRun(string(b[match[1]]), fonts[match[0]])
		}
		oi, ni = match[0]+1, match[1]+1
	}
	flush()
	next.Rich = rich
	return next
}

func sameRichFont(a, b *RichFont) bool {
	if a == nil || b == nil {
		return a == b
	}
	return sameOptional(a.Name, b.Name) && sameOptional(a.Size, b.Size) &&
		sameOptional(a.Color, b.Color) && sameOptional(a.Bold, b.Bold) &&
		sameOptional(a.Italic, b.Italic) && sameOptional(a.Underline, b.Underline) &&
		sameOptional(a.Strike, b.Strike)
}

func sameOptional[T comparable](a, b *T) bool {
	return a == b || a != nil && b != nil && *a == *b
}

// richTextMatches maps unchanged Unicode characters across a commit, including
// multiple edits made before pressing Enter. Prefix/suffix trimming makes the
// common single-edit case linear. Bound the table to 16 MiB; unusually large
// changed regions are treated as one replacement with the same inheritance rule.
func richTextMatches(a, b []rune) [][2]int {
	var matches [][2]int
	start := 0
	for start < min(len(a), len(b)) && a[start] == b[start] {
		matches = append(matches, [2]int{start, start})
		start++
	}
	endA, endB := len(a), len(b)
	for endA > start && endB > start && a[endA-1] == b[endB-1] {
		endA--
		endB--
	}
	n, m := endA-start, endB-start
	if n > 0 && m > 0 && n+1 <= 4_000_000/(m+1) {
		width := m + 1
		dp := make([]uint32, (n+1)*width)
		for i := n - 1; i >= 0; i-- {
			for j := m - 1; j >= 0; j-- {
				if a[start+i] == b[start+j] {
					dp[i*width+j] = dp[(i+1)*width+j+1] + 1
				} else {
					dp[i*width+j] = max(dp[(i+1)*width+j], dp[i*width+j+1])
				}
			}
		}
		for i, j := 0, 0; i < n && j < m; {
			if a[start+i] == b[start+j] {
				matches = append(matches, [2]int{start + i, start + j})
				i++
				j++
			} else if dp[(i+1)*width+j] >= dp[i*width+j+1] {
				i++
			} else {
				j++
			}
		}
	}
	for endA < len(a) {
		matches = append(matches, [2]int{endA, endB})
		endA++
		endB++
	}
	return matches
}
