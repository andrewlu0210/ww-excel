package eval

import (
	"strings"
	"unicode"

	"ww-excel/internal/model"
)

// Criteria is a SUMIF/COUNTIF condition such as 5, ">=10", "<>", "台*".
type Criteria struct {
	op      string // = <> < > <= >=
	num     float64
	isNum   bool
	isBool  bool
	text    string
	pattern []rune // lower-cased wildcard pattern for = and <> on text
	wild    bool
}

func ParseCriteria(v Value) Criteria {
	switch v.Kind {
	case KNumber:
		return Criteria{op: "=", num: v.Num, isNum: true}
	case KBool:
		return Criteria{op: "=", num: v.Num, isBool: true}
	case KEmpty:
		return Criteria{op: "=", text: ""}
	}
	s, _ := ToText(v)
	c := Criteria{op: "="}
	for _, op := range []string{"<=", ">=", "<>", "<", ">", "="} {
		if strings.HasPrefix(s, op) {
			c.op = op
			s = s[len(op):]
			break
		}
	}
	if n, ok := model.ParseNumber(s); ok {
		c.num, c.isNum = n, true
		return c
	}
	switch strings.ToUpper(s) {
	case "TRUE":
		c.num, c.isBool = 1, true
		return c
	case "FALSE":
		c.num, c.isBool = 0, true
		return c
	}
	c.text = s
	c.pattern = []rune(strings.ToLower(s))
	c.wild = strings.ContainsAny(s, "*?~")
	return c
}

// Match tests a cell value. Empty cells are passed as Empty.
func (c Criteria) Match(v Value) bool {
	if v.Kind == KError {
		// Errors only match an equal error criterion.
		return c.op == "=" && !c.isNum && !c.isBool && strings.EqualFold(v.Str, c.text)
	}
	switch {
	case c.isNum:
		var n float64
		switch v.Kind {
		case KNumber:
			n = v.Num
		case KString:
			// "=5" also matches the text "5", but other operators only
			// compare numbers.
			p, ok := model.ParseNumber(v.Str)
			if !ok || c.op != "=" && c.op != "<>" {
				return c.op == "<>"
			}
			n = p
		default:
			return c.op == "<>"
		}
		return cmpOp(c.op, Compare(Number(n), Number(c.num)))
	case c.isBool:
		if v.Kind != KBool {
			return c.op == "<>"
		}
		return cmpOp(c.op, cmpInt(int(v.Num), int(c.num)))
	}

	// Text criteria.
	if c.text == "" {
		empty := v.Kind == KEmpty || v.Kind == KString && v.Str == ""
		if c.op == "<>" {
			return !empty
		}
		if c.op == "=" {
			return empty
		}
	}
	switch c.op {
	case "=", "<>":
		// Text criteria never match numbers or logicals; "<>abc" matches
		// everything that is not "abc".
		eq := v.Kind == KString && wildMatch(c.pattern, []rune(strings.ToLower(v.Str)), c.wild)
		if c.op == "=" {
			return eq
		}
		return !eq
	default:
		if v.Kind != KString {
			return false
		}
		return cmpOp(c.op, Compare(v, Text(c.text)))
	}
}

func cmpOp(op string, c int) bool {
	switch op {
	case "=":
		return c == 0
	case "<>":
		return c != 0
	case "<":
		return c < 0
	case ">":
		return c > 0
	case "<=":
		return c <= 0
	default:
		return c >= 0
	}
}

// WildMatch matches s against an Excel wildcard pattern (* ? and ~
// escapes), case-insensitively.
func WildMatch(pattern, s string) bool {
	return wildMatch([]rune(strings.ToLower(pattern)), []rune(strings.ToLower(s)), true)
}

func wildMatch(p, s []rune, wild bool) bool {
	if !wild {
		return string(p) == string(s)
	}
	// Iterative matcher with backtracking on the last '*'.
	pi, si := 0, 0
	star, mark := -1, 0
	for si < len(s) {
		switch {
		case pi < len(p) && p[pi] == '~' && pi+1 < len(p):
			if unicode.ToLower(p[pi+1]) == s[si] {
				pi += 2
				si++
				continue
			}
		case pi < len(p) && (p[pi] == '?' || p[pi] == s[si]) && p[pi] != '*':
			pi++
			si++
			continue
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, si
			pi++
			continue
		}
		if star >= 0 {
			pi = star + 1
			mark++
			si = mark
			continue
		}
		return false
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}
