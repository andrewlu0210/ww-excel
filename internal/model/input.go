package model

import (
	"strconv"
	"strings"
)

// ParseInput converts text typed into a cell into a Cell, following
// Excel's basic rules: "=..." is a formula (compiled by the engine),
// numbers, TRUE/FALSE and error codes become values, a leading
// apostrophe forces text, and empty input clears the cell.
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

// ParseValue converts imported text (e.g. a CSV field) into a Cell the
// way Excel does when opening a CSV: formulas, numbers, TRUE/FALSE and
// error codes are recognized; everything else stays literal text.
func ParseValue(text string) Cell {
	if text == "" {
		return Cell{}
	}
	if rest, ok := strings.CutPrefix(text, "="); ok && strings.TrimSpace(rest) != "" {
		return FormulaCell(rest)
	}
	if n, ok := parseNumber(text); ok {
		return NumberCell(n)
	}
	switch strings.ToUpper(strings.TrimSpace(text)) {
	case "TRUE":
		return BoolCell(true)
	case "FALSE":
		return BoolCell(false)
	}
	if code := ErrorCode(strings.TrimSpace(text)); code != "" {
		return ErrorCell(code)
	}
	return StringCell(text)
}

// ParseNumber parses text the way Excel coerces text to a number in
// formulas (" 12 " -> 12; "1e3" -> 1000; "abc" -> false).
func ParseNumber(text string) (float64, bool) { return parseNumber(text) }

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

// EditText returns the text shown when a cell is edited: the formula for
// formula cells, otherwise the value. Text that would parse back as
// something else gets an apostrophe so editing round-trips.
func (c Cell) EditText() string {
	if c.F != nil {
		return "=" + c.F.Text
	}
	if c.Kind == KindString {
		if parsed := ParseValue(c.Str); parsed.Kind != KindString || parsed.F != nil || strings.HasPrefix(c.Str, "'") {
			return "'" + c.Str
		}
	}
	return c.Display()
}
