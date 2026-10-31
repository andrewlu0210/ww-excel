package xlsxfile

import (
	"encoding/xml"
	"io"
	"strconv"

	"ww-excel/internal/model"
)

// Isolate one RawToken subtree for Decode without changing the worksheet
// stream's token mode or reading past the closing element.
type rawElementTokens struct {
	source *xml.Decoder
	first  *xml.StartElement
	depth  int
	done   bool
}

func (r *rawElementTokens) Token() (xml.Token, error) {
	if r.done {
		return nil, io.EOF
	}
	var token xml.Token
	var err error
	if r.first != nil {
		token = *r.first
		r.first = nil
	} else {
		token, err = r.source.RawToken()
	}
	if err != nil {
		return nil, err
	}
	switch token.(type) {
	case xml.StartElement:
		r.depth++
	case xml.EndElement:
		r.depth--
		if r.depth == 0 {
			r.done = true
		}
	}
	return token, nil
}
func decodeRawElement(dec *xml.Decoder, start xml.StartElement, out any) error {
	return xml.NewTokenDecoder(&rawElementTokens{source: dec, first: &start}).Decode(out)
}

type rawCFColor struct {
	RGB   string  `xml:"rgb,attr"`
	Theme *int    `xml:"theme,attr"`
	Tint  float64 `xml:"tint,attr"`
}
type rawCFThreshold struct {
	Type    string `xml:"type,attr"`
	Value   string `xml:"val,attr"`
	Formula string `xml:"f"`
	GTE     *bool  `xml:"gte,attr"`
}

func (v rawCFThreshold) model() model.CFThreshold {
	value := v.Value
	if value == "" {
		value = v.Formula
	}
	return model.CFThreshold{Type: v.Type, Value: value, Strict: v.GTE != nil && !*v.GTE}
}

type rawCFDataBar struct {
	Thresholds  []rawCFThreshold `xml:"cfvo"`
	Color       *rawCFColor      `xml:"color"`
	BorderColor *rawCFColor      `xml:"borderColor"`
	Gradient    *bool            `xml:"gradient,attr"`
	ShowValue   *bool            `xml:"showValue,attr"`
	Border      *bool            `xml:"border,attr"`
}

type rawConditionalGroup struct {
	Range     string `xml:"sqref,attr"`
	RangeText string `xml:"sqref"`
	Rules     []struct {
		Attrs    []xml.Attr    `xml:",any,attr"`
		Formulas []string      `xml:"formula"`
		IDs      []string      `xml:"extLst>ext>id"`
		Colors   []rawCFColor  `xml:"colorScale>color"`
		Bar      *rawCFDataBar `xml:"dataBar"`
		Icons    *struct {
			Set        string           `xml:"iconSet,attr"`
			Reverse    bool             `xml:"reverse,attr"`
			ShowValue  *bool            `xml:"showValue,attr"`
			Thresholds []rawCFThreshold `xml:"cfvo"`
			Icons      []struct {
				Set   string `xml:"iconSet,attr"`
				Index int    `xml:"iconId,attr"`
			} `xml:"cfIcon"`
		} `xml:"iconSet"`
	} `xml:"cfRule"`
}

// Decode both the original worksheet rules and x14 extensions.
func (r *reader) readConditionalGroup(group rawConditionalGroup) []rawRule {
	ranges := group.Range
	if ranges == "" {
		ranges = group.RangeText
	}
	color := func(c rawCFColor) string { return resolveColor(r.f, c.RGB, c.Theme, c.Tint) }
	var out []rawRule
	for _, source := range group.Rules {
		el := xml.StartElement{Attr: source.Attrs}
		rule := rawRule{rule: cfRuleFromAttrs(el), dxf: -1, formulas: source.Formulas, id: attr(el, "id")}
		rule.priority, _ = strconv.Atoi(attr(el, "priority"))
		if id := attr(el, "dxfId"); id != "" {
			rule.dxf, _ = strconv.Atoi(id)
		}
		if rule.id == "" && len(source.IDs) > 0 {
			rule.id = source.IDs[0]
		}
		rule.rule.Ranges = parseSqref(ranges)
		for _, c := range source.Colors {
			rule.colors = append(rule.colors, color(c))
		}
		if b := source.Bar; b != nil {
			rule.bar = b
			bar := &model.CFDataBar{Solid: b.Gradient != nil && !*b.Gradient, HideValue: b.ShowValue != nil && !*b.ShowValue}
			if len(b.Thresholds) >= 2 {
				bar.Minimum = b.Thresholds[0].model()
				bar.Maximum = b.Thresholds[1].model()
			}
			if b.Color != nil {
				rule.colors = []string{color(*b.Color)}
			}
			if b.Border != nil && *b.Border && b.BorderColor != nil {
				bar.BorderColor = color(*b.BorderColor)
			}
			rule.rule.DataBar = bar
		}
		if icons := source.Icons; icons != nil {
			set := &model.CFIconSet{Set: icons.Set, Reverse: icons.Reverse, HideValue: icons.ShowValue != nil && !*icons.ShowValue}
			if set.Set == "" {
				set.Set = "3TrafficLights1"
			}
			for _, v := range icons.Thresholds {
				set.Thresholds = append(set.Thresholds, v.model())
			}
			for _, icon := range icons.Icons {
				set.Icons = append(set.Icons, model.CFIcon{Set: icon.Set, Index: icon.Index})
			}
			rule.rule.Type = "icon_set"
			rule.rule.IconSet = set
		}
		out = append(out, rule)
	}
	return out
}

// x14 data bars augment their legacy rule, identified by the shared UUID.
func mergeExtendedConditions(rules []rawRule) []rawRule {
	var out []rawRule
	byID := map[string]int{}
	for _, rule := range rules {
		if index, ok := byID[rule.id]; ok && rule.id != "" {
			base := &out[index]
			if rule.rule.DataBar != nil {
				if base.rule.DataBar == nil {
					base.rule.DataBar = rule.rule.DataBar
				} else if b := rule.bar; b != nil {
					// An omitted extension attribute inherits the legacy setting.
					merged := *base.rule.DataBar
					if len(b.Thresholds) > 0 {
						merged.Minimum = b.Thresholds[0].model()
					}
					if len(b.Thresholds) > 1 {
						merged.Maximum = b.Thresholds[1].model()
					}
					if b.Gradient != nil {
						merged.Solid = !*b.Gradient
					}
					if b.ShowValue != nil {
						merged.HideValue = !*b.ShowValue
					}
					if b.Border != nil {
						if !*b.Border {
							merged.BorderColor = ""
						} else if b.BorderColor != nil {
							merged.BorderColor = rule.rule.DataBar.BorderColor
						}
					}
					base.rule.DataBar = &merged
				}
			}
			if len(rule.colors) > 0 {
				base.colors = rule.colors
			}
			continue
		}
		if rule.id != "" {
			byID[rule.id] = len(out)
		}
		out = append(out, rule)
	}
	return out
}
