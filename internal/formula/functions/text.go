package functions

import (
	"strings"
	"unicode"

	"ww-excel/internal/formula/eval"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// Excel limits text in a cell to 32,767 characters.
const maxText = 32767

func init() {
	reg(catText, "LEN", 1, 1, "text", "傳回文字的字元數", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		return eval.Number(float64(len([]rune(s))))
	})
	reg(catText, "LEFT", 1, 2, "text, [num_chars]", "傳回文字開頭的字元", func(a *eval.Args) eval.Value {
		rs, n, err := textCount(a)
		if err.IsErr() {
			return err
		}
		return eval.Text(string(rs[:min(n, len(rs))]))
	})
	reg(catText, "RIGHT", 1, 2, "text, [num_chars]", "傳回文字結尾的字元", func(a *eval.Args) eval.Value {
		rs, n, err := textCount(a)
		if err.IsErr() {
			return err
		}
		return eval.Text(string(rs[max(0, len(rs)-n):]))
	})
	reg(catText, "MID", 3, 3, "text, start_num, num_chars", "從指定位置傳回指定數量的字元", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		start, err := a.Int(1, 1)
		if err.IsErr() {
			return err
		}
		n, err := a.Int(2, 0)
		if err.IsErr() {
			return err
		}
		if start < 1 || n < 0 {
			return errValue
		}
		rs := []rune(s)
		if start > len(rs) {
			return eval.Text("")
		}
		return eval.Text(string(rs[start-1 : min(len(rs), start-1+n)]))
	})
	reg(catText, "TRIM", 1, 1, "text", "移除前後空格，並將中間多個空格縮成一個", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		parts := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' })
		return eval.Text(strings.Join(parts, " "))
	})
	reg(catText, "UPPER", 1, 1, "text", "將文字轉成大寫", textMap(strings.ToUpper))
	reg(catText, "LOWER", 1, 1, "text", "將文字轉成小寫", textMap(strings.ToLower))
	reg(catText, "PROPER", 1, 1, "text", "將每個英文單字的第一個字母轉成大寫", textMap(proper))
	reg(catText, "SUBSTITUTE", 3, 4, "text, old_text, new_text, [instance_num]", "以新文字取代舊文字", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		old, err := a.Text(1, "")
		if err.IsErr() {
			return err
		}
		repl, err := a.Text(2, "")
		if err.IsErr() {
			return err
		}
		if old == "" {
			return eval.Text(s)
		}
		if !a.Has(3) {
			return limitText(strings.ReplaceAll(s, old, repl))
		}
		nth, err := a.Int(3, 0)
		if err.IsErr() {
			return err
		}
		if nth < 1 {
			return errValue
		}
		idx := -1
		for i, from := 0, 0; i < nth; i++ {
			j := strings.Index(s[from:], old)
			if j < 0 {
				return eval.Text(s)
			}
			idx = from + j
			from = idx + len(old)
		}
		return limitText(s[:idx] + repl + s[idx+len(old):])
	})
	reg(catText, "REPLACE", 4, 4, "old_text, start_num, num_chars, new_text", "取代文字中指定位置的字元", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		start, err := a.Int(1, 1)
		if err.IsErr() {
			return err
		}
		n, err := a.Int(2, 0)
		if err.IsErr() {
			return err
		}
		repl, err := a.Text(3, "")
		if err.IsErr() {
			return err
		}
		if start < 1 || n < 0 {
			return errValue
		}
		rs := []rune(s)
		from := min(start-1, len(rs))
		to := min(from+n, len(rs))
		return limitText(string(rs[:from]) + repl + string(rs[to:]))
	})
	reg(catText, "FIND", 2, 3, "find_text, within_text, [start_num]", "傳回文字的位置（區分大小寫）", func(a *eval.Args) eval.Value {
		return findText(a, false)
	})
	reg(catText, "SEARCH", 2, 3, "find_text, within_text, [start_num]", "傳回文字的位置（不分大小寫，可用萬用字元）", func(a *eval.Args) eval.Value {
		return findText(a, true)
	})
	reg(catText, "VALUE", 1, 1, "text", "將文字轉換成數字", func(a *eval.Args) eval.Value {
		v := a.Scalar(0)
		if v.Kind != eval.KString {
			n, err := eval.ToNumber(v)
			if err.IsErr() {
				return err
			}
			return eval.Number(n)
		}
		s := strings.TrimSpace(v.Str)
		if p, ok := strings.CutSuffix(s, "%"); ok {
			if n, ok := model.ParseNumber(p); ok {
				return eval.Number(n / 100)
			}
		}
		if n, ok := model.ParseNumber(strings.ReplaceAll(s, ",", "")); ok {
			return eval.Number(n)
		}
		return errValue
	})
	reg(catText, "TEXT", 2, 2, "value, format_text", "依格式碼將數字轉成文字", func(a *eval.Args) eval.Value {
		v := a.Scalar(0)
		if v.IsErr() {
			return v
		}
		code, err := a.Text(1, "")
		if err.IsErr() {
			return err
		}
		switch v.Kind {
		case eval.KNumber, eval.KEmpty:
			return eval.Text(numfmt.Format(v.Num, code))
		case eval.KString:
			if n, ok := model.ParseNumber(v.Str); ok {
				return eval.Text(numfmt.Format(n, code))
			}
			return eval.Text(numfmt.FormatText(v.Str, code))
		}
		s, _ := eval.ToText(v)
		return eval.Text(s)
	})
	reg(catText, "CONCAT", 1, variadic, "text1, [text2], ...", "合併多個文字或範圍", func(a *eval.Args) eval.Value {
		var b strings.Builder
		var errv eval.Value
		a.EachValue(0, func(v eval.Value, _ bool) bool {
			s, err := eval.ToText(v)
			if err.IsErr() {
				errv = err
				return false
			}
			b.WriteString(s)
			return true
		})
		if errv.IsErr() {
			return errv
		}
		return limitText(b.String())
	})
	reg(catText, "CONCATENATE", 1, variadic, "text1, [text2], ...", "合併多個文字", func(a *eval.Args) eval.Value {
		var b strings.Builder
		for i := 0; i < a.Len(); i++ {
			s, err := a.Text(i, "")
			if err.IsErr() {
				return err
			}
			b.WriteString(s)
		}
		return limitText(b.String())
	})
	reg(catText, "TEXTJOIN", 3, variadic, "delimiter, ignore_empty, text1, ...", "以分隔符號合併多個文字或範圍", func(a *eval.Args) eval.Value {
		sep, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		ignore, err := a.Bool(1, true)
		if err.IsErr() {
			return err
		}
		var parts []string
		var errv eval.Value
		a.EachValue(2, func(v eval.Value, _ bool) bool {
			s, err := eval.ToText(v)
			if err.IsErr() {
				errv = err
				return false
			}
			if s != "" || !ignore {
				parts = append(parts, s)
			}
			return true
		})
		if errv.IsErr() {
			return errv
		}
		return limitText(strings.Join(parts, sep))
	})
	reg(catText, "REPT", 2, 2, "text, number_times", "將文字重複指定次數", func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		n, err := a.Int(1, 0)
		if err.IsErr() {
			return err
		}
		if n < 0 || len([]rune(s))*n > maxText {
			return errValue
		}
		return eval.Text(strings.Repeat(s, n))
	})
	reg(catText, "EXACT", 2, 2, "text1, text2", "比較兩個文字是否完全相同（區分大小寫）", func(a *eval.Args) eval.Value {
		x, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		y, err := a.Text(1, "")
		if err.IsErr() {
			return err
		}
		return eval.Bool(x == y)
	})
}

func textMap(f func(string) string) eval.Fn {
	return func(a *eval.Args) eval.Value {
		s, err := a.Text(0, "")
		if err.IsErr() {
			return err
		}
		return eval.Text(f(s))
	}
}

func proper(s string) string {
	rs := []rune(s)
	prevLetter := false
	for i, r := range rs {
		if prevLetter {
			rs[i] = unicode.ToLower(r)
		} else {
			rs[i] = unicode.ToUpper(r)
		}
		prevLetter = unicode.IsLetter(r)
	}
	return string(rs)
}

func textCount(a *eval.Args) ([]rune, int, eval.Value) {
	s, err := a.Text(0, "")
	if err.IsErr() {
		return nil, 0, err
	}
	n, err := a.Int(1, 1)
	if err.IsErr() {
		return nil, 0, err
	}
	if n < 0 {
		return nil, 0, errValue
	}
	return []rune(s), n, eval.Empty
}

func limitText(s string) eval.Value {
	if len([]rune(s)) > maxText {
		return errValue
	}
	return eval.Text(s)
}

func findText(a *eval.Args, search bool) eval.Value {
	needle, err := a.Text(0, "")
	if err.IsErr() {
		return err
	}
	hay, err := a.Text(1, "")
	if err.IsErr() {
		return err
	}
	start, err := a.Int(2, 1)
	if err.IsErr() {
		return err
	}
	rs := []rune(hay)
	if start < 1 || start > len(rs)+1 {
		return errValue
	}
	if needle == "" {
		return eval.Number(float64(start))
	}
	for i := start - 1; i < len(rs); i++ {
		rest := string(rs[i:])
		if search {
			if eval.WildMatch(needle+"*", rest) {
				return eval.Number(float64(i + 1))
			}
		} else if strings.HasPrefix(rest, needle) {
			return eval.Number(float64(i + 1))
		}
	}
	return errValue
}
