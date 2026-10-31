package model

// Validation is a data validation rule (資料驗證) on some ranges.
type Validation struct {
	Ranges []Range
	// Type: list, whole, decimal, date, time, textLength, custom.
	Type string
	// Operator for numeric types: between, notBetween, equal, notEqual,
	// greaterThan, lessThan, greaterThanOrEqual, lessThanOrEqual.
	Operator string
	// Formulas without "=". For a list, Formula1 is either a quoted
	// comma-separated list ("是,否") or a reference ($H$1:$H$5).
	Formula1, Formula2 string
	AllowBlank         bool
	NoDropdown         bool // list: hide the in-cell dropdown
	// ErrorStyle: stop (default), warning, information.
	ErrorStyle          string
	ErrorTitle, Error   string
	PromptTitle, Prompt string
}

// CFThreshold is a numeric, percentage, percentile or formula boundary.
type CFThreshold struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Strict bool   `json:"strict,omitempty"`
}
type CFDataBar struct {
	Minimum     CFThreshold `json:"minimum"`
	Maximum     CFThreshold `json:"maximum"`
	Solid       bool        `json:"solid"`
	BorderColor string      `json:"borderColor,omitempty"`
	HideValue   bool        `json:"hideValue,omitempty"`
}
type CFIcon struct {
	Set   string `json:"set"`
	Index int    `json:"index"`
}
type CFIconSet struct {
	Set        string        `json:"set"`
	Reverse    bool          `json:"reverse,omitempty"`
	HideValue  bool          `json:"hideValue,omitempty"`
	Thresholds []CFThreshold `json:"thresholds"`
	Icons      []CFIcon      `json:"icons,omitempty"`
}

// CFRule is one conditional formatting rule. Rules are kept in
// priority order; the first matching rule wins for each property.
type CFRule struct {
	Priority int
	DataBar  *CFDataBar
	IconSet  *CFIconSet
	Ranges   []Range
	// Type: cell, formula, top, bottom, average, duplicate, unique, text,
	// blanks, no_blanks, errors, no_errors, 2_color_scale,
	// 3_color_scale, data_bar.
	Type string
	// Op for cell (between, greaterThan, ...) and text (containsText,
	// notContains, beginsWith, endsWith) rules.
	Op string
	// Value1/Value2: formulas without "=" for cell rules, the formula
	// for formula rules, the text for text rules, the rank for top and
	// bottom.
	Value1, Value2 string
	Percent        bool // top/bottom: rank is a percentage
	Below          bool // average: below instead of above
	Format         CFFormat
	// Colors for scales (min, [mid], max) and data bars (bar color),
	// as RRGGBB.
	Colors     []string
	StopIfTrue bool
}

// CFFormat is the formatting a matching rule applies.
type CFFormat struct {
	Color     string // font color RRGGBB
	Fill      string
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
}

// shiftRanges maps ranges through an insertion or deletion, dropping
// ranges that were deleted entirely.
func shiftRanges(rs []Range, rows bool, at, count int) []Range {
	var out []Range
	for _, r := range rs {
		lo, hi, limit := r.C0, r.C1, MaxCols
		if rows {
			lo, hi, limit = r.R0, r.R1, MaxRows
		}
		nlo, nhi, ok := ShiftSpan(lo, hi, at, count, limit)
		if !ok {
			continue
		}
		if rows {
			r.R0, r.R1 = nlo, nhi
		} else {
			r.C0, r.C1 = nlo, nhi
		}
		out = append(out, r)
	}
	return out
}

func cloneValidations(vs []Validation) []Validation {
	out := make([]Validation, len(vs))
	for i, v := range vs {
		v.Ranges = append([]Range(nil), v.Ranges...)
		out[i] = v
	}
	return out
}

func cloneCF(rs []CFRule) []CFRule {
	out := make([]CFRule, len(rs))
	for i, r := range rs {
		r.Ranges = append([]Range(nil), r.Ranges...)
		r.Colors = append([]string(nil), r.Colors...)
		if r.DataBar != nil {
			bar := *r.DataBar
			r.DataBar = &bar
		}
		if r.IconSet != nil {
			icons := *r.IconSet
			icons.Thresholds = append([]CFThreshold(nil), icons.Thresholds...)
			icons.Icons = append([]CFIcon(nil), icons.Icons...)
			r.IconSet = &icons
		}
		out[i] = r
	}
	return out
}

// Subtract returns the parts of r outside o (up to four rectangles).
func (r Range) Subtract(o Range) []Range {
	if !r.Overlaps(o) {
		return []Range{r}
	}
	var out []Range
	if r.R0 < o.R0 {
		out = append(out, Range{R0: r.R0, C0: r.C0, R1: o.R0 - 1, C1: r.C1})
	}
	if r.R1 > o.R1 {
		out = append(out, Range{R0: o.R1 + 1, C0: r.C0, R1: r.R1, C1: r.C1})
	}
	top, bottom := max(r.R0, o.R0), min(r.R1, o.R1)
	if r.C0 < o.C0 {
		out = append(out, Range{R0: top, C0: r.C0, R1: bottom, C1: o.C0 - 1})
	}
	if r.C1 > o.C1 {
		out = append(out, Range{R0: top, C0: o.C1 + 1, R1: bottom, C1: r.C1})
	}
	return out
}

// InRanges reports whether a cell lies in any of the ranges.
func InRanges(rs []Range, row, col int) bool {
	for _, r := range rs {
		if r.Contains(row, col) {
			return true
		}
	}
	return false
}
