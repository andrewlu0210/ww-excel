package document

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/model"
)

// CFResult is what conditional formatting does to one cell.
type CFResult struct {
	Icon           *model.CFIcon
	BarSolid       bool
	BarBorderColor string
	HideValue      bool
	Format         model.CFFormat // merged formats of the matching rules
	// Fill from a color scale (overrides Format.Fill), RRGGBB.
	ScaleFill string
	// Data bar length (0 to 1), and its color; Bar < 0 means none.
	Bar      float64
	BarColor string
}

// CFEval evaluates a sheet's conditional formats. Range statistics
// (top 10, averages, duplicates, scales) are computed once.
type CFEval struct {
	d       *Document
	sheet   int
	display DisplayFunc
	rules   []*cfPrepared
}

type cfPrepared struct {
	rule *model.CFRule
	// Numbers in the rule's ranges, sorted.
	nums      []float64
	avg       float64
	threshold float64
	counts    map[string]int
	lo, mid   float64
	hi        float64
}

// CondFormats prepares the sheet's rules, or returns nil if it has none.
func (d *Document) CondFormats(sheet int, display DisplayFunc) *CFEval {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil
	}
	rules := d.wb.Sheets[sheet].Layout.CondFormats
	if len(rules) == 0 {
		return nil
	}
	ev := &CFEval{d: d, sheet: sheet, display: display}
	for i := range rules {
		ev.rules = append(ev.rules, ev.prepare(&rules[i]))
	}
	return ev
}

// Ranges lists every range with a rule, for finding cells to format.
func (e *CFEval) Ranges() []model.Range {
	var out []model.Range
	for _, p := range e.rules {
		out = append(out, p.rule.Ranges...)
	}
	return out
}

// eachIn visits non-empty cells of the ranges, clamped to the used area.
func (e *CFEval) eachIn(ranges []model.Range, fn func(row, col int, c model.Cell)) {
	sh := e.d.wb.Sheets[e.sheet]
	rows, cols := sh.UsedRange()
	for _, r := range ranges {
		r.R1, r.C1 = min(r.R1, rows-1), min(r.C1, cols-1)
		if r.R1 < r.R0 || r.C1 < r.C0 {
			continue
		}
		sh.EachInRange(r, fn)
	}
}

func (e *CFEval) prepare(r *model.CFRule) *cfPrepared {
	p := &cfPrepared{rule: r}
	switch r.Type {
	case "top", "bottom", "average", "2_color_scale", "3_color_scale", "data_bar", "icon_set":
		e.eachIn(r.Ranges, func(_, _ int, c model.Cell) {
			if c.Kind == model.KindNumber {
				p.nums = append(p.nums, c.Num)
			}
		})
		sort.Float64s(p.nums)
		if n := len(p.nums); n > 0 {
			p.avg = sum(p.nums) / float64(n)
			p.lo, p.hi = p.nums[0], p.nums[n-1]
			p.mid = percentile(p.nums, 0.5)
			rank, _ := strconv.Atoi(strings.TrimSpace(r.Value1))
			if rank <= 0 {
				rank = 10
			}
			if r.Percent {
				rank = max(1, int(math.Floor(float64(n)*float64(rank)/100)))
			}
			rank = min(rank, n)
			if r.Type == "top" {
				p.threshold = p.nums[n-rank]
			} else {
				p.threshold = p.nums[rank-1]
			}
		}
	case "duplicate", "unique":
		p.counts = map[string]int{}
		sh := e.d.wb.Sheets[e.sheet]
		e.eachIn(r.Ranges, func(row, col int, c model.Cell) {
			if c.HasContent() {
				p.counts[strings.ToLower(e.display(sh, row, col, c))]++
			}
		})
	}
	return p
}

func sum(xs []float64) float64 {
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t
}

func percentile(sorted []float64, q float64) float64 {
	pos := q * float64(len(sorted)-1)
	i := int(pos)
	if i >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	return sorted[i] + (pos-float64(i))*(sorted[i+1]-sorted[i])
}

// At returns the formatting for a cell; ok is false when no rule
// applies.
func (e *CFEval) At(row, col int, c model.Cell) (CFResult, bool) {
	res := CFResult{Bar: -1}
	var merged model.CFFormat
	matched := false
	iconApplied := false
	for _, p := range e.rules {
		r := p.rule
		if !model.InRanges(r.Ranges, row, col) {
			continue
		}
		switch r.Type {
		case "2_color_scale", "3_color_scale":
			if c.Kind == model.KindNumber && res.ScaleFill == "" && len(p.nums) > 0 {
				res.ScaleFill = scaleColor(r, p, c.Num)
				matched = true
			}
			continue
		case "data_bar":
			if c.Kind == model.KindNumber && res.Bar < 0 && len(p.nums) > 0 {
				lo := math.Min(0, p.lo)
				hi := math.Max(0, p.hi)
				if bar := r.DataBar; bar != nil {
					if bound, ok := e.cfThreshold(p, bar.Minimum, row, col); ok {
						lo = bound
					}
					if bound, ok := e.cfThreshold(p, bar.Maximum, row, col); ok {
						hi = bound
					}
					res.BarSolid, res.BarBorderColor = bar.Solid, bar.BorderColor
					res.HideValue = res.HideValue || bar.HideValue
				}
				if hi > lo {
					res.Bar = math.Max(0, math.Min(1, (c.Num-lo)/(hi-lo)))
				} else {
					res.Bar = 1
				}
				res.BarColor = "638EC6"
				if len(r.Colors) > 0 && r.Colors[0] != "" {
					res.BarColor = r.Colors[0]
				}
				matched = true
			}
			continue
		case "icon_set":
			if c.Kind == model.KindNumber && r.IconSet != nil && len(r.IconSet.Thresholds) > 0 && !iconApplied {
				iconApplied = true
				set := r.IconSet
				index := 0
				for i := 1; i < len(set.Thresholds); i++ {
					threshold := set.Thresholds[i]
					bound, ok := e.cfThreshold(p, threshold, row, col)
					if ok && (c.Num > bound || c.Num == bound && !threshold.Strict) {
						index = i
					}
				}
				if set.Reverse {
					index = len(set.Thresholds) - 1 - index
				}
				icon := model.CFIcon{Set: set.Set, Index: index}
				if index >= 0 && index < len(set.Icons) {
					icon = set.Icons[index]
				}
				if icon.Set != "NoIcons" {
					res.Icon = &icon
				}
				res.HideValue = res.HideValue || set.HideValue
				matched = true
			}
			continue
		}
		if !e.matches(p, row, col, c) {
			continue
		}
		matched = true
		mergeCF(&merged, r.Format)
		if r.StopIfTrue {
			break
		}
	}
	res.Format = merged
	return res, matched
}

// mergeCF fills properties of dst not set by a higher-priority rule.
func mergeCF(dst *model.CFFormat, src model.CFFormat) {
	if dst.Color == "" {
		dst.Color = src.Color
	}
	if dst.Fill == "" {
		dst.Fill = src.Fill
	}
	dst.Bold = dst.Bold || src.Bold
	dst.Italic = dst.Italic || src.Italic
	dst.Underline = dst.Underline || src.Underline
	dst.Strike = dst.Strike || src.Strike
}

func (e *CFEval) cfThreshold(p *cfPrepared, t model.CFThreshold, row, col int) (float64, bool) {
	switch t.Type {
	case "autoMin":
		return math.Min(0, p.lo), len(p.nums) > 0
	case "min":
		return p.lo, len(p.nums) > 0
	case "autoMax":
		return math.Max(0, p.hi), len(p.nums) > 0
	case "max":
		return p.hi, len(p.nums) > 0
	case "num":
		v, err := strconv.ParseFloat(t.Value, 64)
		return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
	case "percent", "percentile":
		v, err := strconv.ParseFloat(t.Value, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || len(p.nums) == 0 {
			return 0, false
		}
		v = math.Max(0, math.Min(100, v))
		if t.Type == "percentile" {
			return percentile(p.nums, v/100), true
		}
		return p.lo + (p.hi-p.lo)*v/100, true
	case "formula":
		node, err := ruleFormula(t.Value, p.rule.Ranges, row, col)
		if err != nil {
			return 0, false
		}
		value := e.d.calc.EvalAt(node, e.sheet, row, col)
		if value.Kind == eval.KNumber && !math.IsNaN(value.Num) && !math.IsInf(value.Num, 0) {
			return value.Num, true
		}
	}
	return 0, false
}

func (e *CFEval) matches(p *cfPrepared, row, col int, c model.Cell) bool {
	r := p.rule
	sh := e.d.wb.Sheets[e.sheet]
	switch r.Type {
	case "cell":
		v := eval.FromCell(c)
		if v.IsErr() {
			return false
		}
		a, ok := e.value(r.Value1, r, row, col)
		if !ok {
			return false
		}
		cmp := eval.Compare(v, a)
		switch r.Op {
		case "between", "notBetween":
			b, ok := e.value(r.Value2, r, row, col)
			if !ok {
				return false
			}
			lo, hi := a, b
			if eval.Compare(lo, hi) > 0 {
				lo, hi = hi, lo
			}
			in := eval.Compare(v, lo) >= 0 && eval.Compare(v, hi) <= 0
			return in == (r.Op == "between")
		case "equal":
			return cmp == 0
		case "notEqual":
			return cmp != 0
		case "greaterThan":
			return cmp > 0
		case "lessThan":
			return cmp < 0
		case "greaterThanOrEqual":
			return cmp >= 0
		case "lessThanOrEqual":
			return cmp <= 0
		}
	case "formula":
		node, err := ruleFormula(r.Value1, r.Ranges, row, col)
		if err != nil {
			return false
		}
		ok, errv := eval.ToBool(e.d.calc.EvalAt(node, e.sheet, row, col))
		return ok && !errv.IsErr()
	case "top", "bottom":
		if c.Kind != model.KindNumber || len(p.nums) == 0 {
			return false
		}
		if r.Type == "top" {
			return c.Num >= p.threshold
		}
		return c.Num <= p.threshold
	case "average":
		if c.Kind != model.KindNumber || len(p.nums) == 0 {
			return false
		}
		if r.Below {
			return c.Num < p.avg
		}
		return c.Num > p.avg
	case "duplicate", "unique":
		if !c.HasContent() {
			return false
		}
		n := p.counts[strings.ToLower(e.display(sh, row, col, c))]
		return (n > 1) == (r.Type == "duplicate")
	case "text":
		text := strings.ToLower(e.display(sh, row, col, c))
		needle := strings.ToLower(r.Value1)
		switch r.Op {
		case "notContains":
			return !strings.Contains(text, needle)
		case "beginsWith":
			return strings.HasPrefix(text, needle)
		case "endsWith":
			return strings.HasSuffix(text, needle)
		}
		return c.HasContent() && strings.Contains(text, needle)
	case "blanks", "no_blanks":
		blank := !c.HasContent() || c.Kind == model.KindString && strings.TrimSpace(c.Str) == ""
		return blank == (r.Type == "blanks")
	case "errors", "no_errors":
		return (c.Kind == model.KindError) == (r.Type == "errors")
	}
	return false
}

// value evaluates a cell rule's bound at (row, col).
func (e *CFEval) value(text string, r *model.CFRule, row, col int) (eval.Value, bool) {
	if strings.TrimSpace(text) == "" {
		return eval.Value{}, false
	}
	node, err := ruleFormula(text, r.Ranges, row, col)
	if err != nil {
		return eval.Text(text), true
	}
	v := e.d.calc.EvalAt(node, e.sheet, row, col)
	return v, !v.IsErr()
}

func scaleColor(r *model.CFRule, p *cfPrepared, x float64) string {
	colors := r.Colors
	if len(colors) < 2 {
		colors = []string{"F8696B", "FFEB84", "63BE7B"}
		if r.Type == "2_color_scale" {
			colors = []string{"FFFFFF", "63BE7B"}
		}
	}
	if r.Type == "3_color_scale" && len(colors) >= 3 {
		if x <= p.mid {
			return mixColor(colors[0], colors[1], frac(x, p.lo, p.mid))
		}
		return mixColor(colors[1], colors[2], frac(x, p.mid, p.hi))
	}
	return mixColor(colors[0], colors[len(colors)-1], frac(x, p.lo, p.hi))
}

func frac(x, lo, hi float64) float64 {
	if hi <= lo {
		return 1
	}
	return math.Max(0, math.Min(1, (x-lo)/(hi-lo)))
}

func mixColor(a, b string, t float64) string {
	pa, pb := parseRGB(a), parseRGB(b)
	var out [3]int
	for i := range out {
		out[i] = int(math.Round(float64(pa[i]) + (float64(pb[i])-float64(pa[i]))*t))
	}
	return fmt.Sprintf("%02X%02X%02X", out[0], out[1], out[2])
}

func parseRGB(s string) [3]int {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 8 {
		s = s[2:] // ARGB
	}
	n, _ := strconv.ParseUint(s, 16, 32)
	return [3]int{int(n >> 16 & 255), int(n >> 8 & 255), int(n & 255)}
}

// AddCFRuleCommand adds a rule for ranges with the highest priority.
func (d *Document) AddCFRuleCommand(sheet int, ranges []model.Range, rule model.CFRule) (command.Command, error) {
	if err := checkCFRule(&rule); err != nil {
		return nil, err
	}
	rule.Ranges = append([]model.Range(nil), ranges...)
	return d.layoutCommand(sheet, "設定格式化的條件", nil, func(l *model.Layout) {
		l.CondFormats = append([]model.CFRule{rule}, l.CondFormats...)
	})
}

// DeleteCFRuleCommand removes the rule at index.
func (d *Document) DeleteCFRuleCommand(sheet, index int) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) || index < 0 || index >= len(d.wb.Sheets[sheet].Layout.CondFormats) {
		return nil, fmt.Errorf("規則不存在")
	}
	return d.layoutCommand(sheet, "刪除格式化規則", nil, func(l *model.Layout) {
		l.CondFormats = append(l.CondFormats[:index:index], l.CondFormats[index+1:]...)
	})
}

// ClearCFCommand removes rules from ranges (all rules when ranges is
// empty).
func (d *Document) ClearCFCommand(sheet int, ranges []model.Range) (command.Command, error) {
	if sheet < 0 || sheet >= len(d.wb.Sheets) || len(d.wb.Sheets[sheet].Layout.CondFormats) == 0 {
		return nil, nil
	}
	return d.layoutCommand(sheet, "清除格式化的條件", nil, func(l *model.Layout) {
		if len(ranges) == 0 {
			l.CondFormats = nil
			return
		}
		var out []model.CFRule
		for _, r := range l.CondFormats {
			rs := r.Ranges
			for _, sel := range ranges {
				var next []model.Range
				for _, x := range rs {
					next = append(next, x.Subtract(sel)...)
				}
				rs = next
			}
			if len(rs) > 0 {
				r.Ranges = rs
				out = append(out, r)
			}
		}
		l.CondFormats = out
	})
}

func checkCFRule(r *model.CFRule) error {
	switch r.Type {
	case "cell", "formula":
		if strings.TrimSpace(r.Value1) == "" {
			return fmt.Errorf("請輸入條件的值或公式")
		}
		r.Value1 = strings.TrimPrefix(strings.TrimSpace(r.Value1), "=")
		r.Value2 = strings.TrimPrefix(strings.TrimSpace(r.Value2), "=")
		if _, err := ruleFormula(r.Value1, nil, 0, 0); err != nil {
			return fmt.Errorf("公式有誤：%v", err)
		}
	case "top", "bottom", "average", "duplicate", "unique", "text", "blanks", "no_blanks", "errors", "no_errors",
		"2_color_scale", "3_color_scale", "data_bar", "icon_set":
	default:
		return fmt.Errorf("未知的規則類型「%s」", r.Type)
	}
	return nil
}

// adjustRuleFormulas rewrites the validation and conditional format
// formulas of a sheet (after rows or columns moved).
func adjustRuleFormulas(sh *model.Sheet, fn func(ast.Node) (ast.Node, bool)) {
	fix := func(text string) string {
		if text == "" || strings.HasPrefix(strings.TrimSpace(text), `"`) {
			return text
		}
		node, err := ruleFormula(text, nil, 0, 0)
		if err != nil {
			return text
		}
		if out, ok := fn(node); ok {
			return ast.Format(out)
		}
		return text
	}
	for i := range sh.Layout.Validations {
		v := &sh.Layout.Validations[i]
		v.Formula1, v.Formula2 = fix(v.Formula1), fix(v.Formula2)
	}
	for i := range sh.Layout.CondFormats {
		r := &sh.Layout.CondFormats[i]
		if r.Type == "cell" || r.Type == "formula" {
			r.Value1, r.Value2 = fix(r.Value1), fix(r.Value2)
		}
		if r.DataBar != nil {
			if r.DataBar.Minimum.Type == "formula" {
				r.DataBar.Minimum.Value = fix(r.DataBar.Minimum.Value)
			}
			if r.DataBar.Maximum.Type == "formula" {
				r.DataBar.Maximum.Value = fix(r.DataBar.Maximum.Value)
			}
		}
		if r.IconSet != nil {
			for j := range r.IconSet.Thresholds {
				threshold := &r.IconSet.Thresholds[j]
				if threshold.Type == "formula" {
					threshold.Value = fix(threshold.Value)
				}
			}
		}
	}
}
