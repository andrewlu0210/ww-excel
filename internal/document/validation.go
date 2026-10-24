package document

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"ww-excel/internal/command"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/eval"
	"ww-excel/internal/formula/parser"
	"ww-excel/internal/model"
)

// ValidationAt returns the validation rule of a cell, or nil.
func (d *Document) ValidationAt(sheet, row, col int) *model.Validation {
	if sheet < 0 || sheet >= len(d.wb.Sheets) {
		return nil
	}
	vs := d.wb.Sheets[sheet].Layout.Validations
	for i := range vs {
		if model.InRanges(vs[i].Ranges, row, col) {
			return &vs[i]
		}
	}
	return nil
}

// ruleFormula parses a rule formula and moves its relative references
// from the top-left of the rule's first range to (row, col), as Excel
// does for validation and conditional format formulas.
func ruleFormula(text string, ranges []model.Range, row, col int) (ast.Node, error) {
	node, err := parser.Parse(strings.TrimPrefix(text, "="))
	if err != nil {
		return nil, err
	}
	if len(ranges) > 0 {
		node = ast.Translate(node, row-ranges[0].R0, col-ranges[0].C0)
	}
	return node, nil
}

// ListValues returns the choices of a list validation at a cell, as
// display text.
func (d *Document) ListValues(sheet, row, col int, display DisplayFunc) []string {
	v := d.ValidationAt(sheet, row, col)
	if v == nil || v.Type != "list" {
		return nil
	}
	return d.listValues(v, sheet, row, col, display)
}

func (d *Document) listValues(v *model.Validation, sheet, row, col int, display DisplayFunc) []string {
	f := strings.TrimSpace(v.Formula1)
	if strings.HasPrefix(f, `"`) && strings.HasSuffix(f, `"`) && len(f) >= 2 {
		var out []string
		for _, item := range strings.Split(strings.ReplaceAll(f[1:len(f)-1], `""`, `"`), ",") {
			out = append(out, strings.TrimSpace(item))
		}
		return out
	}
	node, err := ruleFormula(f, v.Ranges, row, col)
	if err != nil {
		return nil
	}
	val := d.calc.ReferenceAt(node, sheet, row, col)
	if val.Kind != eval.KRef {
		if t, e := eval.ToText(d.calc.EvalAt(node, sheet, row, col)); !e.IsErr() && t != "" {
			return []string{t}
		}
		return nil
	}
	r := val.Ref
	if r.Sheet < 0 || r.Sheet >= len(d.wb.Sheets) {
		return nil
	}
	sh := d.wb.Sheets[r.Sheet]
	var out []string
	rg := r.Range()
	if rows, _ := sh.UsedRange(); rg.R1 >= rows {
		rg.R1 = max(rg.R0, rows-1)
	}
	for rr := rg.R0; rr <= rg.R1 && len(out) < 10000; rr++ {
		for cc := rg.C0; cc <= rg.C1; cc++ {
			if c := sh.Get(rr, cc); c.HasContent() {
				out = append(out, display(sh, rr, cc, c))
			}
		}
	}
	return out
}

// ValidationError is input refused by data validation.
type ValidationError struct {
	Title, Message string
	// Stop: the value cannot be entered. Otherwise (warning,
	// information) it is entered and the message is a notice.
	Stop bool
}

func (e *ValidationError) Error() string { return e.Message }

// Validate checks a value about to be entered at a cell.
func (d *Document) Validate(sheet, row, col int, cell model.Cell, display DisplayFunc) *ValidationError {
	v := d.ValidationAt(sheet, row, col)
	if v == nil || d.validValue(v, sheet, row, col, cell, display) {
		return nil
	}
	msg := v.Error
	if msg == "" {
		msg = "這個值不符合此儲存格定義的資料驗證限制。"
	}
	return &ValidationError{Title: v.ErrorTitle, Message: msg, Stop: v.ErrorStyle == "" || v.ErrorStyle == "stop"}
}

func (d *Document) validValue(v *model.Validation, sheet, row, col int, cell model.Cell, display DisplayFunc) bool {
	if !cell.HasContent() || cell.Kind == model.KindString && cell.Str == "" && cell.F == nil {
		return true // Excel always accepts clearing a cell
	}
	val := eval.FromCell(cell)
	switch v.Type {
	case "list":
		sh := d.wb.Sheets[sheet]
		text := display(sh, row, col, cell)
		for _, item := range d.listValues(v, sheet, row, col, display) {
			if strings.EqualFold(item, text) {
				return true
			}
		}
		return false
	case "custom":
		node, err := ruleFormula(v.Formula1, v.Ranges, row, col)
		if err != nil {
			return true
		}
		// The formula sees the new value.
		sh := d.wb.Sheets[sheet]
		old := sh.Get(row, col)
		sh.Set(row, col, cell)
		res := d.calc.EvalAt(node, sheet, row, col)
		sh.Set(row, col, old)
		ok, e := eval.ToBool(res)
		return ok && !e.IsErr()
	case "textLength":
		t, _ := eval.ToText(val)
		return d.compare(v, sheet, row, col, float64(utf8.RuneCountInString(t)))
	case "whole", "decimal", "date", "time":
		if val.Kind != eval.KNumber {
			return false
		}
		if v.Type == "whole" && val.Num != float64(int64(val.Num)) {
			return false
		}
		return d.compare(v, sheet, row, col, val.Num)
	}
	return true
}

// compare checks x against the rule's operator and bounds.
func (d *Document) compare(v *model.Validation, sheet, row, col int, x float64) bool {
	bound := func(text string) (float64, bool) {
		if text == "" {
			return 0, false
		}
		node, err := ruleFormula(text, v.Ranges, row, col)
		if err != nil {
			return 0, false
		}
		n, e := eval.ToNumber(d.calc.EvalAt(node, sheet, row, col))
		return n, !e.IsErr()
	}
	a, okA := bound(v.Formula1)
	b, okB := bound(v.Formula2)
	if !okA {
		return true
	}
	switch v.Operator {
	case "", "between":
		return okB && x >= min(a, b) && x <= max(a, b)
	case "notBetween":
		return okB && (x < min(a, b) || x > max(a, b))
	case "equal":
		return x == a
	case "notEqual":
		return x != a
	case "greaterThan":
		return x > a
	case "lessThan":
		return x < a
	case "greaterThanOrEqual":
		return x >= a
	case "lessThanOrEqual":
		return x <= a
	}
	return true
}

// SetValidationCommand applies a rule to ranges (nil clears them),
// taking those cells out of any other rule.
func (d *Document) SetValidationCommand(sheet int, ranges []model.Range, v *model.Validation) (command.Command, error) {
	if v != nil {
		if err := checkValidation(v); err != nil {
			return nil, err
		}
	}
	name := "資料驗證"
	if v == nil {
		name = "清除資料驗證"
	}
	return d.layoutCommand(sheet, name, nil, func(l *model.Layout) {
		var out []model.Validation
		for _, old := range l.Validations {
			rs := old.Ranges
			for _, sel := range ranges {
				var next []model.Range
				for _, r := range rs {
					next = append(next, r.Subtract(sel)...)
				}
				rs = next
			}
			if len(rs) > 0 {
				old.Ranges = rs
				out = append(out, old)
			}
		}
		if v != nil {
			nv := *v
			nv.Ranges = append([]model.Range(nil), ranges...)
			out = append(out, nv)
		}
		l.Validations = out
	})
}

func checkValidation(v *model.Validation) error {
	switch v.Type {
	case "list", "whole", "decimal", "date", "time", "textLength", "custom":
	default:
		return fmt.Errorf("未知的驗證類型「%s」", v.Type)
	}
	if strings.TrimSpace(v.Formula1) == "" {
		return errors.New("請輸入來源或數值")
	}
	for _, f := range []string{v.Formula1, v.Formula2} {
		if f == "" || v.Type == "list" && strings.HasPrefix(strings.TrimSpace(f), `"`) {
			continue
		}
		if _, err := parser.Parse(strings.TrimPrefix(strings.TrimSpace(f), "=")); err != nil {
			return fmt.Errorf("公式有誤：%v", err)
		}
	}
	return nil
}
