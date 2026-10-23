package ast

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Format prints n as canonical formula text (without the leading "="):
// upper-case function names and references, no extra spaces.
func Format(n Node) string {
	var b strings.Builder
	write(&b, n)
	return b.String()
}

func write(b *strings.Builder, n Node) {
	switch x := n.(type) {
	case Number:
		b.WriteString(FormatNumber(x.V))
	case String:
		b.WriteByte('"')
		b.WriteString(strings.ReplaceAll(x.V, `"`, `""`))
		b.WriteByte('"')
	case Bool:
		if x.V {
			b.WriteString("TRUE")
		} else {
			b.WriteString("FALSE")
		}
	case Error:
		b.WriteString(x.Code)
	case Missing:
	case Paren:
		b.WriteByte('(')
		write(b, x.X)
		b.WriteByte(')')
	case Unary:
		b.WriteByte(x.Op)
		write(b, x.X)
	case Percent:
		write(b, x.X)
		b.WriteByte('%')
	case Binary:
		write(b, x.L)
		b.WriteString(x.Op)
		write(b, x.R)
	case Call:
		b.WriteString(x.Name)
		b.WriteByte('(')
		for i, a := range x.Args {
			if i > 0 {
				b.WriteByte(',')
			}
			write(b, a)
		}
		b.WriteByte(')')
	case Array:
		b.WriteByte('{')
		for i, row := range x.Rows {
			if i > 0 {
				b.WriteByte(';')
			}
			for j, el := range row {
				if j > 0 {
					b.WriteByte(',')
				}
				write(b, el)
			}
		}
		b.WriteByte('}')
	case Name:
		b.WriteString(x.Name)
	case Ref:
		if x.Sheet != "" {
			b.WriteString(QuoteSheet(x.Sheet))
			b.WriteByte('!')
		}
		b.WriteString(FormatArea(x.Area))
	}
}

// FormatNumber prints a number literal the way Excel stores it.
func FormatNumber(v float64) string {
	abs := math.Abs(v)
	if abs != 0 && (abs >= 1e21 || abs < 1e-6) {
		return strings.ToUpper(strconv.FormatFloat(v, 'E', -1, 64))
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func FormatArea(a Area) string {
	dollar := func(abs bool) string {
		if abs {
			return "$"
		}
		return ""
	}
	col := func(c int, abs bool) string { return dollar(abs) + ColName(c) }
	row := func(r int, abs bool) string { return dollar(abs) + strconv.Itoa(r+1) }

	switch a.Kind {
	case ColArea:
		return col(a.C0, a.AbsC0) + ":" + col(a.C1, a.AbsC1)
	case RowArea:
		return row(a.R0, a.AbsR0) + ":" + row(a.R1, a.AbsR1)
	}
	s := col(a.C0, a.AbsC0) + row(a.R0, a.AbsR0)
	if a.IsSingle() {
		return s
	}
	return s + ":" + col(a.C1, a.AbsC1) + row(a.R1, a.AbsR1)
}

// QuoteSheet wraps a sheet name in quotes when Excel requires it:
// anything but letters, digits and underscores, a leading digit, or a
// name that reads like a cell reference.
func QuoteSheet(name string) string {
	needs := name == ""
	for i, r := range name {
		if !(unicode.IsLetter(r) || r == '_' || (i > 0 && (unicode.IsDigit(r) || r == '.'))) {
			needs = true
			break
		}
	}
	if !needs && looksLikeCell(name) {
		needs = true
	}
	if !needs {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func looksLikeCell(s string) bool {
	i := 0
	for i < len(s) && (s[i] >= 'A' && s[i] <= 'Z' || s[i] >= 'a' && s[i] <= 'z') {
		i++
	}
	if i == 0 || i == len(s) {
		return false
	}
	for j := i; j < len(s); j++ {
		if s[j] < '0' || s[j] > '9' {
			return false
		}
	}
	return i <= 3
}
