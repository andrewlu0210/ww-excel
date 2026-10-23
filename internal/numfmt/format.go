package numfmt

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

type tokKind int

const (
	tLiteral tokKind = iota
	tDigit           // 0 # ?
	tPoint           // decimal point
	tComma           // thousands separator or scaling, resolved later
	tPercent
	tExp     // E+ / E-
	tDate    // y m d h s AM/PM tokens; text holds the code
	tText    // @
	tGeneral // General
)

type token struct {
	kind tokKind
	text string // literal text, digit placeholder char, date code, exp sign
}

type condition struct {
	op  string
	val float64
}

type section struct {
	tokens []token
	isDate bool
	cond   *condition
	// Set when the section prints its own sign, e.g. "(0.0)".
	hasText bool
}

var monthNames = []string{"January", "February", "March", "April", "May", "June", "July",
	"August", "September", "October", "November", "December"}
var dayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// Format renders a number with an Excel format code.
func Format(v float64, code string) string {
	secs := parse(code)
	sec, useAbs := pick(secs, v)
	if sec == nil {
		return General(v)
	}
	if useAbs {
		v = math.Abs(v)
	}
	if sec.isDate {
		return formatDate(v, sec.tokens)
	}
	return formatNumber(v, sec.tokens, !useAbs)
}

// FormatText renders a text value: the fourth section (or a lone
// section containing @) substitutes the text for @.
func FormatText(s, code string) string {
	secs := parse(code)
	var sec *section
	if len(secs) >= 4 {
		sec = &secs[3]
	} else if len(secs) == 1 && containsKind(secs[0].tokens, tText) {
		sec = &secs[0]
	}
	if sec == nil {
		return s
	}
	var b strings.Builder
	for _, t := range sec.tokens {
		switch t.kind {
		case tText:
			b.WriteString(s)
		case tLiteral:
			b.WriteString(t.text)
		}
	}
	return b.String()
}

func containsKind(ts []token, k tokKind) bool {
	for _, t := range ts {
		if t.kind == k {
			return true
		}
	}
	return false
}

// pick chooses the section for v. useAbs means the section is a
// dedicated negative section and v's sign must not be printed.
func pick(secs []section, v float64) (*section, bool) {
	if len(secs) == 0 {
		return nil, false
	}
	// Explicit conditions: [>100]0;[<0]-0;0
	if secs[0].cond != nil {
		for i := range secs {
			if c := secs[i].cond; c == nil || c.match(v) {
				return &secs[i], i > 0 && v < 0 && secs[i].cond == nil
			}
		}
		return &secs[len(secs)-1], false
	}
	switch {
	case v < 0 && len(secs) >= 2:
		return &secs[1], true
	case v == 0 && len(secs) >= 3:
		return &secs[2], false
	}
	return &secs[0], false
}

func (c *condition) match(v float64) bool {
	switch c.op {
	case "<":
		return v < c.val
	case "<=":
		return v <= c.val
	case ">":
		return v > c.val
	case ">=":
		return v >= c.val
	case "<>":
		return v != c.val
	default:
		return v == c.val
	}
}

func parse(code string) []section {
	var secs []section
	cur := section{}
	rs := []rune(code)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == ';':
			secs = append(secs, cur)
			cur = section{}
		case r == '"':
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				j++
			}
			cur.lit(string(rs[i+1 : min(j, len(rs))]))
			i = j
		case r == '\\' && i+1 < len(rs):
			i++
			cur.lit(string(rs[i]))
		case r == '_' && i+1 < len(rs):
			i++
			cur.lit(" ")
		case r == '*' && i+1 < len(rs):
			i++ // fill character: not supported, skip
		case r == '[':
			j := i + 1
			for j < len(rs) && rs[j] != ']' {
				j++
			}
			cur.bracket(string(rs[i+1 : min(j, len(rs))]))
			i = j
		case r == '0' || r == '#' || r == '?':
			cur.add(tDigit, string(r))
		case r == '.':
			cur.add(tPoint, ".")
		case r == ',':
			cur.add(tComma, ",")
		case r == '%':
			cur.add(tPercent, "%")
		case (r == 'E' || r == 'e') && i+1 < len(rs) && (rs[i+1] == '+' || rs[i+1] == '-'):
			cur.add(tExp, string(rs[i+1]))
			i++
		case r == '@':
			cur.add(tText, "@")
		case hasPrefixFold(rs[i:], "General"):
			cur.add(tGeneral, "")
			i += len("General") - 1
		case hasPrefixFold(rs[i:], "AM/PM"):
			cur.date("AM/PM")
			i += 4
		case hasPrefixFold(rs[i:], "A/P"):
			cur.date("A/P")
			i += 2
		case strings.ContainsRune("ymdhsYMDHS", r):
			j := i
			for j < len(rs) && unicode.ToLower(rs[j]) == unicode.ToLower(r) {
				j++
			}
			cur.date(strings.ToLower(string(rs[i:j])))
			i = j - 1
		default:
			cur.lit(string(r))
		}
	}
	return append(secs, cur)
}

func hasPrefixFold(rs []rune, prefix string) bool {
	p := []rune(prefix)
	if len(rs) < len(p) {
		return false
	}
	return strings.EqualFold(string(rs[:len(p)]), prefix)
}

func (s *section) add(k tokKind, text string) {
	s.tokens = append(s.tokens, token{k, text})
}

func (s *section) lit(text string) {
	s.hasText = true
	if n := len(s.tokens); n > 0 && s.tokens[n-1].kind == tLiteral {
		s.tokens[n-1].text += text
		return
	}
	s.add(tLiteral, text)
}

func (s *section) date(code string) {
	s.isDate = true
	s.add(tDate, code)
}

func (s *section) bracket(content string) {
	lower := strings.ToLower(content)
	switch {
	case lower == "h" || lower == "hh" || lower == "m" || lower == "mm" || lower == "s" || lower == "ss":
		s.date("[" + lower + "]")
	case strings.HasPrefix(content, "<") || strings.HasPrefix(content, ">") || strings.HasPrefix(content, "="):
		op := content[:1]
		rest := content[1:]
		if strings.HasPrefix(rest, "=") || (op == "<" && strings.HasPrefix(rest, ">")) {
			op += rest[:1]
			rest = rest[1:]
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(rest), 64); err == nil {
			s.cond = &condition{op: op, val: v}
		}
	default:
		// Colors ([Red]) and locale tags ([$-404]) are ignored for now.
	}
}

// ---- Numbers ----

func formatNumber(v float64, toks []token, signed bool) string {
	if containsKind(toks, tGeneral) {
		var b strings.Builder
		for _, t := range toks {
			if t.kind == tGeneral {
				if signed {
					b.WriteString(General(v))
				} else {
					b.WriteString(General(math.Abs(v)))
				}
			} else if t.kind == tLiteral {
				b.WriteString(t.text)
			}
		}
		return b.String()
	}
	if !containsKind(toks, tDigit) {
		// Literal-only section, e.g. "zero".
		var b strings.Builder
		for _, t := range toks {
			if t.kind == tLiteral {
				b.WriteString(t.text)
			}
		}
		return b.String()
	}

	// Classify commas and find the parts of the pattern.
	pointAt, expAt := -1, -1
	for i, t := range toks {
		if t.kind == tPoint && pointAt < 0 && expAt < 0 {
			pointAt = i
		}
		if t.kind == tExp && expAt < 0 {
			expAt = i
		}
	}
	intEnd := len(toks)
	if pointAt >= 0 {
		intEnd = pointAt
	} else if expAt >= 0 {
		intEnd = expAt
	}

	// A comma between integer digit placeholders groups thousands; a
	// comma after the last digit placeholder scales by 1000; anything
	// else prints as a literal comma.
	mantEnd := len(toks)
	if expAt >= 0 {
		mantEnd = expAt
	}
	thousands := false
	scale := 0
	literalComma := make(map[int]bool)
	for i, t := range toks {
		if t.kind != tComma {
			continue
		}
		switch {
		case i < intEnd && nextDigitBefore(toks, i+1, intEnd) && nextDigitBefore(toks, 0, i):
			thousands = true
		case !nextDigitBefore(toks, i+1, mantEnd):
			scale++
		default:
			literalComma[i] = true
		}
	}
	percents := 0
	for _, t := range toks {
		if t.kind == tPercent {
			percents++
		}
	}

	x := v
	if signed {
		x = math.Abs(v)
	}
	x /= math.Pow(1000, float64(scale))
	x *= math.Pow(100, float64(percents))

	var intPh, fracPh, expPh []string
	for i, t := range toks {
		if t.kind != tDigit {
			continue
		}
		switch {
		case expAt >= 0 && i > expAt:
			expPh = append(expPh, t.text)
		case i < intEnd:
			intPh = append(intPh, t.text)
		default:
			fracPh = append(fracPh, t.text)
		}
	}

	exponent := 0
	if expAt >= 0 && x != 0 {
		lead := max(1, len(intPh))
		exponent = int(math.Floor(math.Log10(x))) - (lead - 1)
		x /= math.Pow(10, float64(exponent))
		// Rounding may push the mantissa up a digit (9.99 -> 10.0).
		if Round(x, len(fracPh)) >= math.Pow(10, float64(lead)) {
			exponent++
			x /= 10
		}
	}

	x = Round(x, len(fracPh))
	intDigits, fracDigits := splitDigits(x, len(fracPh))
	if intDigits == "0" {
		intDigits = ""
	}

	intOut := renderInt(intDigits, intPh, thousands)
	fracOut := renderFrac(fracDigits, fracPh)

	var b strings.Builder
	if signed && v < 0 && x != 0 {
		b.WriteByte('-')
	}
	intDone, fracIdx := false, 0
	for i, t := range toks {
		switch t.kind {
		case tLiteral:
			b.WriteString(t.text)
		case tPercent:
			b.WriteByte('%')
		case tComma:
			if literalComma[i] {
				b.WriteByte(',')
			}
		case tPoint:
			if i == pointAt {
				b.WriteByte('.')
			}
		case tExp:
			b.WriteByte('E')
			sign := "+"
			if exponent < 0 {
				sign = "-"
			} else if t.text == "-" {
				sign = ""
			}
			e := strconv.Itoa(abs(exponent))
			for len(e) < len(expPh) {
				e = "0" + e
			}
			b.WriteString(sign + e)
		case tDigit:
			switch {
			case expAt >= 0 && i > expAt:
				// written with the exponent
			case i < intEnd:
				if !intDone {
					b.WriteString(intOut)
					intDone = true
				}
			default:
				if fracIdx < len(fracOut) {
					b.WriteString(fracOut[fracIdx])
				}
				fracIdx++
			}
		}
	}
	return b.String()
}

func nextDigitBefore(toks []token, from, end int) bool {
	for i := from; i < end; i++ {
		if toks[i].kind == tDigit {
			return true
		}
	}
	return false
}

func splitDigits(x float64, decimals int) (string, string) {
	s := strconv.FormatFloat(x, 'f', decimals, 64)
	i, f, _ := strings.Cut(s, ".")
	return i, f
}

// renderInt pads the integer digits for the placeholders: '0' pads with
// zeros, '?' with spaces, '#' with nothing.
func renderInt(digits string, ph []string, thousands bool) string {
	pad := ""
	for i := 0; i < len(ph)-len(digits); i++ {
		switch ph[i] {
		case "0":
			pad += "0"
		case "?":
			pad += " "
		}
	}
	out := pad + digits
	if thousands {
		out = group(out)
	}
	return out
}

func group(s string) string {
	lead := len(s) - len(strings.TrimLeft(s, " "))
	body := s[lead:]
	if len(body) <= 3 {
		return s
	}
	var b strings.Builder
	for i, r := range body {
		if i > 0 && (len(body)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return s[:lead] + b.String()
}

// renderFrac returns one string per fraction placeholder; trailing zeros
// under '#' vanish and under '?' become spaces.
func renderFrac(digits string, ph []string) []string {
	out := make([]string, len(ph))
	trailing := true
	for i := len(ph) - 1; i >= 0; i-- {
		d := "0"
		if i < len(digits) {
			d = string(digits[i])
		}
		if trailing && d == "0" && ph[i] != "0" {
			if ph[i] == "?" {
				out[i] = " "
			}
			continue
		}
		trailing = false
		out[i] = d
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ---- Dates ----

func formatDate(v float64, toks []token) string {
	if v < 0 {
		return "#######"
	}
	twelve := false
	for _, t := range toks {
		if t.text == "AM/PM" || t.text == "A/P" {
			twelve = true
		}
	}
	y, mo, d := YMD(v)
	h, mi, s := HMS(v)

	var b strings.Builder
	for i, t := range toks {
		switch t.kind {
		case tLiteral, tComma, tPoint, tPercent:
			// Not special in date sections. (Fractional seconds "ss.0"
			// are not supported; the 0 placeholders are dropped.)
			b.WriteString(t.text)
			continue
		case tDate:
		default:
			continue
		}
		switch c := t.text; {
		case c[0] == 'y':
			if len(c) <= 2 {
				b.WriteString(pad2(y % 100))
			} else {
				b.WriteString(strconv.Itoa(y))
			}
		case c[0] == 'm' && isMinute(toks, i):
			if len(c) == 1 {
				b.WriteString(strconv.Itoa(mi))
			} else {
				b.WriteString(pad2(mi))
			}
		case c[0] == 'm':
			switch len(c) {
			case 1:
				b.WriteString(strconv.Itoa(mo))
			case 2:
				b.WriteString(pad2(mo))
			case 3:
				b.WriteString(monthNames[mo-1][:3])
			case 5:
				b.WriteString(monthNames[mo-1][:1])
			default:
				b.WriteString(monthNames[mo-1])
			}
		case c[0] == 'd':
			switch len(c) {
			case 1:
				b.WriteString(strconv.Itoa(d))
			case 2:
				b.WriteString(pad2(d))
			case 3:
				b.WriteString(dayNames[Weekday(v)][:3])
			default:
				b.WriteString(dayNames[Weekday(v)])
			}
		case c[0] == 'h':
			hh := h
			if twelve {
				hh = h % 12
				if hh == 0 {
					hh = 12
				}
			}
			if len(c) == 1 {
				b.WriteString(strconv.Itoa(hh))
			} else {
				b.WriteString(pad2(hh))
			}
		case c[0] == 's':
			if len(c) == 1 {
				b.WriteString(strconv.Itoa(s))
			} else {
				b.WriteString(pad2(s))
			}
		case c == "AM/PM":
			if h < 12 {
				b.WriteString("AM")
			} else {
				b.WriteString("PM")
			}
		case c == "A/P":
			if h < 12 {
				b.WriteString("A")
			} else {
				b.WriteString("P")
			}
		case c == "[h]" || c == "[hh]":
			b.WriteString(strconv.Itoa(int(math.Floor(v*24 + 1e-9))))
		case c == "[m]" || c == "[mm]":
			b.WriteString(strconv.Itoa(int(math.Floor(v*1440 + 1e-9))))
		case c == "[s]" || c == "[ss]":
			b.WriteString(strconv.Itoa(int(math.Round(v * 86400))))
		}
	}
	return b.String()
}

// An m/mm token means minutes when it follows an hour token or precedes
// a seconds token (ignoring literals).
func isMinute(toks []token, i int) bool {
	if len(toks[i].text) > 2 {
		return false
	}
	for j := i - 1; j >= 0; j-- {
		if toks[j].kind == tDate {
			return toks[j].text[0] == 'h' || strings.HasPrefix(toks[j].text, "[h")
		}
	}
	for j := i + 1; j < len(toks); j++ {
		if toks[j].kind == tDate {
			return toks[j].text[0] == 's'
		}
	}
	return false
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
