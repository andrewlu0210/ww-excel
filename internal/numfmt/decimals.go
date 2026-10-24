package numfmt

import (
	"regexp"
	"strings"
)

// The number part of a section: digit placeholders with optional
// thousands separators and decimals.
var reDigits = regexp.MustCompile(`[0#?][0#?,]*(\.[0#?]*)?`)

// AdjustDecimals adds (delta > 0) or removes decimal places in every
// section of a format code, for the 增加/減少小數位數 buttons. General
// becomes "0.0" or "0"; date formats and text are unchanged.
func AdjustDecimals(code string, delta int) string {
	if code == "" || strings.EqualFold(code, "General") {
		if delta > 0 {
			return "0." + strings.Repeat("0", delta)
		}
		return "0"
	}
	if IsDate(code) {
		return code
	}
	sections := splitSections(code)
	for i, sec := range sections {
		sections[i] = adjustSection(sec, delta)
	}
	return strings.Join(sections, ";")
}

func adjustSection(sec string, delta int) string {
	// Skip quoted literals when looking for the digits.
	locs := reDigits.FindAllStringIndex(sec, -1)
	for i := len(locs) - 1; i >= 0; i-- {
		start, end := locs[i][0], locs[i][1]
		if insideQuotes(sec, start) {
			continue
		}
		intPart, frac, _ := strings.Cut(sec[start:end], ".")
		part := intPart
		if n := len(frac) + delta; n > 0 {
			if delta > 0 {
				frac += strings.Repeat("0", delta)
			} else {
				frac = frac[:n]
			}
			part += "." + frac
		}
		return sec[:start] + part + sec[end:]
	}
	return sec
}

func insideQuotes(s string, at int) bool {
	return strings.Count(s[:at], `"`)%2 == 1
}

func splitSections(code string) []string {
	var out []string
	inQuote := false
	start := 0
	for i, r := range code {
		switch r {
		case '"':
			inQuote = !inQuote
		case ';':
			if !inQuote {
				out = append(out, code[start:i])
				start = i + 1
			}
		}
	}
	return append(out, code[start:])
}
