package model

import (
	"strings"
	"unicode"
)

// DefinedName is a named range or formula (名稱). Formula is stored
// without the leading "=", normally an absolute reference such as
// 工作表1!$A$1:$B$5.
type DefinedName struct {
	Name    string
	Scope   *Sheet // nil: the whole workbook
	Formula string
	Comment string
}

// LookupName finds a name visible from sheet: a name scoped to that
// sheet wins over a workbook name. Case-insensitive, like Excel.
func (wb *Workbook) LookupName(name string, sheet *Sheet) *DefinedName {
	var global *DefinedName
	for _, n := range wb.Names {
		if !strings.EqualFold(n.Name, name) {
			continue
		}
		if n.Scope == sheet && sheet != nil {
			return n
		}
		if n.Scope == nil {
			global = n
		}
	}
	return global
}

// ValidName applies Excel's rules for defined names: a letter,
// underscore or backslash first, then letters, digits, periods and
// underscores; not a cell reference; at most 255 characters.
func ValidName(name string) bool {
	rs := []rune(name)
	if len(rs) == 0 || len(rs) > 255 {
		return false
	}
	if !(rs[0] == '_' || rs[0] == '\\' || unicode.IsLetter(rs[0])) {
		return false
	}
	for _, r := range rs[1:] {
		if !(r == '_' || r == '.' || r == '\\' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	upper := strings.ToUpper(name)
	if upper == "R" || upper == "C" || upper == "TRUE" || upper == "FALSE" {
		return false
	}
	return !looksLikeCellRef(upper)
}

// looksLikeCellRef matches A1-style (up to XFD1048576) and R1C1-style
// references, which cannot be names.
func looksLikeCellRef(s string) bool {
	i := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		i++
	}
	if i > 0 && i <= 3 && i < len(s) {
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == len(s) {
			return true
		}
	}
	if strings.HasPrefix(s, "R") {
		rest := strings.TrimLeft(s[1:], "0123456789")
		if rest == "" || strings.HasPrefix(rest, "C") && strings.Trim(rest[1:], "0123456789") == "" {
			return true
		}
	}
	return false
}
