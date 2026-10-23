package model

import (
	"strconv"
	"strings"
)

// ParseInput converts text typed into a cell into a Cell, following
// Excel's basic rules: numbers become numbers, a leading apostrophe
// forces text, and empty input clears the cell.
// Formulas are stored as text until the engine arrives in v0.3.
func ParseInput(text string) Cell {
	if text == "" {
		return Cell{}
	}
	if rest, ok := strings.CutPrefix(text, "'"); ok {
		if rest == "" {
			return Cell{}
		}
		return StringCell(rest)
	}
	return ParseValue(text)
}

// ParseValue converts imported text (e.g. a CSV field) into a Cell:
// numbers become numbers, everything else stays literal text.
func ParseValue(text string) Cell {
	if text == "" {
		return Cell{}
	}
	if n, ok := parseNumber(text); ok {
		return NumberCell(n)
	}
	return StringCell(text)
}

func parseNumber(text string) (float64, bool) {
	s := strings.TrimSpace(text)
	if s == "" {
		return 0, false
	}
	// Reject forms strconv accepts but Excel does not.
	lower := strings.ToLower(s)
	if strings.ContainsAny(lower, "_xpi") || strings.Contains(lower, "nan") {
		return 0, false
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// EditText returns the text shown when a cell is edited. Text that would
// parse back as a number gets an apostrophe so editing round-trips.
func (c Cell) EditText() string {
	if c.Kind == KindString {
		if _, ok := parseNumber(c.Str); ok || strings.HasPrefix(c.Str, "'") {
			return "'" + c.Str
		}
	}
	return c.Display()
}
