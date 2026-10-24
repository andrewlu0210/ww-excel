package xlsxfile

import (
	"html"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"

	"ww-excel/internal/model"
)

// Data validation and conditional formatting IO.

func parseSqref(sqref string) []model.Range {
	var out []model.Range
	for _, part := range strings.Fields(sqref) {
		a, b, found := strings.Cut(part, ":")
		if !found {
			b = a
		}
		c0, r0, err1 := excelize.CellNameToCoordinates(strings.ReplaceAll(a, "$", ""))
		c1, r1, err2 := excelize.CellNameToCoordinates(strings.ReplaceAll(b, "$", ""))
		if err1 == nil && err2 == nil {
			out = append(out, model.Range{R0: r0 - 1, C0: c0 - 1, R1: r1 - 1, C1: c1 - 1}.Normalize())
		}
	}
	return out
}

func formatSqref(rs []model.Range) string {
	parts := make([]string, len(rs))
	for i, r := range rs {
		a, _ := excelize.CoordinatesToCellName(r.C0+1, r.R0+1)
		if r.R0 == r.R1 && r.C0 == r.C1 {
			parts[i] = a
			continue
		}
		b, _ := excelize.CoordinatesToCellName(r.C1+1, r.R1+1)
		parts[i] = a + ":" + b
	}
	return strings.Join(parts, " ")
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func readValidations(f *excelize.File, sheet string) []model.Validation {
	dvs, err := f.GetDataValidations(sheet)
	if err != nil {
		return nil
	}
	var out []model.Validation
	for _, dv := range dvs {
		v := model.Validation{
			Ranges:      parseSqref(dv.Sqref),
			Type:        dv.Type,
			Operator:    dv.Operator,
			Formula1:    strings.TrimPrefix(html.UnescapeString(dv.Formula1), "="),
			Formula2:    strings.TrimPrefix(html.UnescapeString(dv.Formula2), "="),
			AllowBlank:  dv.AllowBlank,
			NoDropdown:  dv.ShowDropDown, // OOXML: showDropDown means hide it
			ErrorStyle:  str(dv.ErrorStyle),
			ErrorTitle:  str(dv.ErrorTitle),
			Error:       str(dv.Error),
			PromptTitle: str(dv.PromptTitle),
			Prompt:      str(dv.Prompt),
		}
		if v.Type == "" || v.Type == "none" || len(v.Ranges) == 0 {
			continue
		}
		out = append(out, v)
	}
	return out
}

// xmlEscape escapes formula text for element content. Quotes stay as
// they are: excelize does not decode &#34; when reading.
func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func writeValidations(f *excelize.File, sheet string, vs []model.Validation) error {
	if err := f.DeleteDataValidation(sheet); err != nil {
		return err
	}
	for _, v := range vs {
		dv := &excelize.DataValidation{
			AllowBlank:       v.AllowBlank,
			Type:             v.Type,
			Operator:         v.Operator,
			Sqref:            formatSqref(v.Ranges),
			Formula1:         xmlEscape(v.Formula1),
			Formula2:         xmlEscape(v.Formula2),
			ShowDropDown:     v.NoDropdown,
			ShowErrorMessage: true,
			ErrorStyle:       ptr(v.ErrorStyle),
			ErrorTitle:       ptr(v.ErrorTitle),
			Error:            ptr(v.Error),
			PromptTitle:      ptr(v.PromptTitle),
			Prompt:           ptr(v.Prompt),
			ShowInputMessage: v.Prompt != "" || v.PromptTitle != "",
		}
		if v.Type == "list" || v.Type == "custom" {
			dv.Operator = ""
		}
		if err := f.AddDataValidation(sheet, dv); err != nil {
			return err
		}
	}
	return nil
}

// Criteria words and symbols excelize uses, mapped to OOXML operators.
var cfOps = map[string]string{
	"!=": "notEqual", "<>": "notEqual", "not equal to": "notEqual", "notEqual": "notEqual",
	"=": "equal", "==": "equal", "equal to": "equal", "equal": "equal",
	">": "greaterThan", "greater than": "greaterThan", "greaterThan": "greaterThan",
	"<": "lessThan", "less than": "lessThan", "lessThan": "lessThan",
	">=": "greaterThanOrEqual", "greater than or equal to": "greaterThanOrEqual", "greaterThanOrEqual": "greaterThanOrEqual",
	"<=": "lessThanOrEqual", "less than or equal to": "lessThanOrEqual", "lessThanOrEqual": "lessThanOrEqual",
	"between": "between", "not between": "notBetween", "notBetween": "notBetween",
	"containing": "containsText", "containsText": "containsText",
	"not containing": "notContains", "notContains": "notContains",
	"begins with": "beginsWith", "beginsWith": "beginsWith",
	"ends with": "endsWith", "endsWith": "endsWith",
}

var cfCriteria = map[string]string{
	"notEqual": "!=", "equal": "==", "greaterThan": ">", "lessThan": "<",
	"greaterThanOrEqual": ">=", "lessThanOrEqual": "<=", "between": "between", "notBetween": "not between",
	"containsText": "containing", "notContains": "not containing", "beginsWith": "begins with", "endsWith": "ends with",
}

func rgb(s string) string {
	s = strings.TrimPrefix(strings.ToUpper(s), "#")
	if len(s) == 8 {
		s = s[2:]
	}
	return s
}

func readCondFormats(f *excelize.File, sheet string) []model.CFRule {
	byRef, err := f.GetConditionalFormats(sheet)
	if err != nil {
		return nil
	}
	refs := make([]string, 0, len(byRef))
	for ref := range byRef {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	var out []model.CFRule
	for _, ref := range refs {
		ranges := parseSqref(ref)
		for _, o := range byRef[ref] {
			r := model.CFRule{Ranges: ranges, Type: o.Type, StopIfTrue: o.StopIfTrue}
			switch o.Type {
			case "cell":
				r.Op = cfOps[o.Criteria]
				r.Value1 = o.Value
				if r.Op == "between" || r.Op == "notBetween" {
					r.Value1, r.Value2 = o.MinValue, o.MaxValue
				}
			case "formula":
				r.Value1 = o.Criteria
				if r.Value1 == "" {
					r.Value1 = o.Value
				}
			case "top", "bottom":
				r.Value1, r.Percent = o.Value, o.Percent
			case "average":
				r.Below = !o.AboveAverage
			case "text":
				r.Op, r.Value1 = cfOps[o.Criteria], o.Value
			case "2_color_scale":
				r.Colors = []string{rgb(o.MinColor), rgb(o.MaxColor)}
			case "3_color_scale":
				r.Colors = []string{rgb(o.MinColor), rgb(o.MidColor), rgb(o.MaxColor)}
			case "data_bar":
				r.Colors = []string{rgb(o.BarColor)}
			case "duplicate", "unique", "blanks", "no_blanks", "errors", "no_errors":
			default:
				continue // icon sets, time periods: not supported (kept only if untouched)
			}
			r.Value1 = strings.TrimPrefix(r.Value1, "=")
			r.Value2 = strings.TrimPrefix(r.Value2, "=")
			if o.Format != nil {
				if st, err := f.GetConditionalStyle(*o.Format); err == nil && st != nil {
					r.Format = cfFormatOf(st)
				}
			}
			out = append(out, r)
		}
	}
	return out
}

func cfFormatOf(st *excelize.Style) model.CFFormat {
	var out model.CFFormat
	if ft := st.Font; ft != nil {
		out.Bold, out.Italic, out.Strike = ft.Bold, ft.Italic, ft.Strike
		out.Underline = ft.Underline != "" && ft.Underline != "none"
		out.Color = rgb(ft.Color)
	}
	if len(st.Fill.Color) > 0 {
		out.Fill = rgb(st.Fill.Color[0])
	}
	return out
}

func writeCondFormats(f *excelize.File, sheet string, rules []model.CFRule) error {
	existing, err := f.GetConditionalFormats(sheet)
	if err != nil {
		return err
	}
	for ref := range existing {
		if err := f.UnsetConditionalFormat(sheet, ref); err != nil {
			return err
		}
	}
	// Excel gives the first rule the highest priority; excelize numbers
	// rules in the order they are added. Rules on the same ranges must
	// be set in one call (a second call replaces the first).
	var order []string
	byRef := map[string][]excelize.ConditionalFormatOptions{}
	for _, r := range rules {
		o := excelize.ConditionalFormatOptions{Type: r.Type, StopIfTrue: r.StopIfTrue, Criteria: "="}
		switch r.Type {
		case "cell":
			o.Criteria = cfCriteria[r.Op]
			o.Value = r.Value1
			if r.Op == "between" || r.Op == "notBetween" {
				o.MinValue, o.MaxValue = r.Value1, r.Value2
			}
		case "formula":
			o.Criteria = r.Value1
		case "top", "bottom":
			o.Value, o.Percent = r.Value1, r.Percent
		case "average":
			o.AboveAverage = !r.Below
		case "text":
			o.Criteria, o.Value = cfCriteria[r.Op], r.Value1
		case "2_color_scale", "3_color_scale":
			o.MinType, o.MaxType = "min", "max"
			colors := r.Colors
			if len(colors) < 2 {
				colors = []string{"F8696B", "FFEB84", "63BE7B"}
				if r.Type == "2_color_scale" {
					colors = []string{"FFFFFF", "63BE7B"}
				}
			}
			colors = append(colors, colors[len(colors)-1])
			o.MinColor = "#" + colors[0]
			if r.Type == "3_color_scale" {
				o.MidType, o.MidValue = "percentile", "50"
				o.MidColor, o.MaxColor = "#"+colors[1], "#"+colors[2]
			} else {
				o.MaxColor = "#" + colors[1]
			}
		case "data_bar":
			o.MinType, o.MaxType = "min", "max"
			o.BarColor = "#638EC6"
			if len(r.Colors) > 0 && r.Colors[0] != "" {
				o.BarColor = "#" + r.Colors[0]
			}
		}
		if hasCFFormat(r.Format) {
			style := &excelize.Style{}
			fm := r.Format
			if fm.Bold || fm.Italic || fm.Strike || fm.Underline || fm.Color != "" {
				style.Font = &excelize.Font{Bold: fm.Bold, Italic: fm.Italic, Strike: fm.Strike, Color: fm.Color}
				if fm.Underline {
					style.Font.Underline = "single"
				}
			}
			if fm.Fill != "" {
				style.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{fm.Fill}}
			}
			id, err := f.NewConditionalStyle(style)
			if err != nil {
				return err
			}
			o.Format = &id
		}
		ref := formatSqref(r.Ranges)
		if _, ok := byRef[ref]; !ok {
			order = append(order, ref)
		}
		byRef[ref] = append(byRef[ref], o)
	}
	for _, ref := range order {
		if err := f.SetConditionalFormat(sheet, ref, byRef[ref]); err != nil {
			return err
		}
	}
	return nil
}

func hasCFFormat(f model.CFFormat) bool { return f != model.CFFormat{} }
