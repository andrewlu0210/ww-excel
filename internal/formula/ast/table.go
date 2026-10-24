package ast

import (
	"errors"
	"strings"
)

// TableSpec is a parsed structured reference specifier.
type TableSpec struct {
	// Row sections; none set means data rows.
	Headers, Data, Totals, ThisRow bool
	// Columns by name; empty means every column. Col1 is "" for one
	// column.
	Col0, Col1 string
}

var errSpec = errors.New("無效的結構化參照")

// ParseTableSpec parses the text inside a structured reference's outer
// brackets: "", "Col", "@Col", "#All", "[#Headers],[Col]",
// "[Col1]:[Col2]", "@[Col Name]" ...
func ParseTableSpec(spec string) (TableSpec, error) {
	var out TableSpec
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return out, nil
	}
	if strings.HasPrefix(spec, "@") {
		out.ThisRow = true
		rest := strings.TrimSpace(spec[1:])
		if rest == "" {
			return out, nil
		}
		if strings.HasPrefix(rest, "[") {
			return out, out.columns(rest, &out)
		}
		out.Col0 = unescape(rest)
		return out, nil
	}
	if !strings.HasPrefix(spec, "[") {
		return out, out.item(spec, &out)
	}
	for _, part := range splitTop(spec, ',') {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "[#") || strings.HasPrefix(part, "[@") {
			inner, ok := bracketed(part)
			if !ok {
				return out, errSpec
			}
			if err := out.item(inner, &out); err != nil {
				return out, err
			}
			continue
		}
		if err := out.columns(part, &out); err != nil {
			return out, err
		}
	}
	return out, nil
}

// columns parses "[Col]" or "[Col1]:[Col2]".
func (TableSpec) columns(part string, out *TableSpec) error {
	ends := splitTop(part, ':')
	if len(ends) > 2 {
		return errSpec
	}
	a, ok := bracketed(strings.TrimSpace(ends[0]))
	if !ok {
		return errSpec
	}
	out.Col0 = unescape(a)
	if len(ends) == 2 {
		b, ok := bracketed(strings.TrimSpace(ends[1]))
		if !ok {
			return errSpec
		}
		out.Col1 = unescape(b)
	}
	return nil
}

// item parses one unbracketed item: a special item or a column name.
func (TableSpec) item(item string, out *TableSpec) error {
	switch strings.ToLower(strings.TrimSpace(item)) {
	case "#all":
		out.Headers, out.Data, out.Totals = true, true, true
	case "#data":
		out.Data = true
	case "#headers":
		out.Headers = true
	case "#totals":
		out.Totals = true
	case "#this row", "@":
		out.ThisRow = true
	default:
		if strings.HasPrefix(item, "#") {
			return errSpec
		}
		out.Col0 = unescape(item)
	}
	return nil
}

func bracketed(s string) (string, bool) {
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return "", false
	}
	return s[1 : len(s)-1], true
}

// splitTop splits s at sep outside brackets.
func splitTop(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			i++
		case '[':
			depth++
		case ']':
			depth--
		case sep:
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

func unescape(s string) string {
	if !strings.Contains(s, "'") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// TableColumnRef builds Table[Col] (escaping special characters).
func TableColumnRef(table, col string) TableRef {
	var b strings.Builder
	for _, r := range col {
		if strings.ContainsRune("[]#'", r) {
			b.WriteByte('\'')
		}
		b.WriteRune(r)
	}
	name := b.String()
	if strings.ContainsAny(col, " ,:") || name != col {
		name = "[" + name + "]"
	}
	return TableRef{Table: table, Spec: name}
}
