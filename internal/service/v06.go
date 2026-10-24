package service

import (
	"context"
	"os/user"
	"strings"
	"time"

	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// ---- Notes ----

type NoteDTO struct {
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

// author is the name new notes are signed with: the macOS user's full
// name, else the login name.
func author() string {
	u, err := user.Current()
	if err != nil {
		return "WW Excel"
	}
	if u.Name != "" {
		return u.Name
	}
	return u.Username
}

// SetNote sets a cell's note; empty text deletes it.
func (s *SheetService) SetNote(ctx context.Context, sheet, row, col int, text string) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.NoteCommand(sheet, row, col, strings.TrimRight(text, "\n"), author())
	})
}

// ---- Data validation ----

type ValidationDTO struct {
	Type        string `json:"type"`
	Operator    string `json:"operator"`
	Formula1    string `json:"formula1"`
	Formula2    string `json:"formula2"`
	AllowBlank  bool   `json:"allowBlank"`
	Dropdown    bool   `json:"dropdown"`
	ErrorStyle  string `json:"errorStyle"`
	ErrorTitle  string `json:"errorTitle"`
	Error       string `json:"error"`
	PromptTitle string `json:"promptTitle"`
	Prompt      string `json:"prompt"`
}

func validationDTO(v *model.Validation) *ValidationDTO {
	if v == nil {
		return nil
	}
	f1 := v.Formula1
	if v.Type == "list" && strings.HasPrefix(f1, `"`) && strings.HasSuffix(f1, `"`) && len(f1) >= 2 {
		f1 = strings.ReplaceAll(f1[1:len(f1)-1], `""`, `"`) // show the plain list
	} else if f1 != "" {
		f1 = "=" + f1
	}
	f2 := v.Formula2
	if f2 != "" {
		f2 = "=" + f2
	}
	return &ValidationDTO{
		Type: v.Type, Operator: v.Operator, Formula1: f1, Formula2: f2, AllowBlank: v.AllowBlank,
		Dropdown: !v.NoDropdown, ErrorStyle: v.ErrorStyle, ErrorTitle: v.ErrorTitle, Error: v.Error,
		PromptTitle: v.PromptTitle, Prompt: v.Prompt,
	}
}

// model turns the dialog's input into a rule: a list source without
// "=" is a literal comma-separated list; numbers need no "=".
func (v ValidationDTO) model() model.Validation {
	clean := func(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "=") }
	f1 := clean(v.Formula1)
	if v.Type == "list" && !strings.HasPrefix(strings.TrimSpace(v.Formula1), "=") {
		f1 = `"` + strings.ReplaceAll(strings.TrimSpace(v.Formula1), `"`, `""`) + `"`
	}
	if v.Type == "date" || v.Type == "time" {
		// Accept typed dates (2026/1/31) as well as formulas.
		f1 = dateBound(v.Formula1)
	}
	f2 := clean(v.Formula2)
	if v.Type == "date" || v.Type == "time" {
		f2 = dateBound(v.Formula2)
	}
	return model.Validation{
		Type: v.Type, Operator: v.Operator, Formula1: f1, Formula2: f2, AllowBlank: v.AllowBlank,
		NoDropdown: !v.Dropdown, ErrorStyle: v.ErrorStyle, ErrorTitle: v.ErrorTitle, Error: v.Error,
		PromptTitle: v.PromptTitle, Prompt: v.Prompt,
	}
}

func dateBound(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "=") {
		return strings.TrimPrefix(s, "=")
	}
	if v, ok := parseDateText(s); ok {
		return v
	}
	return s
}

// Validation returns the rule of a cell, or nil.
func (s *SheetService) Validation(ctx context.Context, sheet, row, col int) *ValidationDTO {
	var out *ValidationDTO
	s.at(ctx).read(func(d *document.Document) { out = validationDTO(d.ValidationAt(sheet, row, col)) })
	return out
}

// ListValues returns the dropdown choices of a cell with a list rule.
func (s *SheetService) ListValues(ctx context.Context, sheet, row, col int) []string {
	out := []string{}
	s.at(ctx).read(func(d *document.Document) {
		if vs := d.ListValues(sheet, row, col, shownText(d.Workbook())); vs != nil {
			out = vs
		}
	})
	return out
}

// SetValidation applies a rule to ranges; nil clears validation there.
func (s *SheetService) SetValidation(ctx context.Context, sheet int, ranges []RangeDTO, v *ValidationDTO) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		if v == nil {
			return d.SetValidationCommand(sheet, toModelRanges(ranges), nil)
		}
		m := v.model()
		return d.SetValidationCommand(sheet, toModelRanges(ranges), &m)
	})
}

// ---- Conditional formatting ----

type CFRuleDTO struct {
	Ranges     []RangeDTO `json:"ranges"`
	Type       string     `json:"type"`
	Op         string     `json:"op"`
	Value1     string     `json:"value1"`
	Value2     string     `json:"value2"`
	Percent    bool       `json:"percent"`
	Below      bool       `json:"below"`
	Color      string     `json:"color"`
	Fill       string     `json:"fill"`
	Bold       bool       `json:"bold"`
	Italic     bool       `json:"italic"`
	Colors     []string   `json:"colors"`
	StopIfTrue bool       `json:"stopIfTrue"`
}

func (r CFRuleDTO) model() model.CFRule {
	return model.CFRule{
		Type: r.Type, Op: r.Op, Value1: r.Value1, Value2: r.Value2, Percent: r.Percent, Below: r.Below,
		Format: model.CFFormat{Color: r.Color, Fill: r.Fill, Bold: r.Bold, Italic: r.Italic},
		Colors: r.Colors, StopIfTrue: r.StopIfTrue,
	}
}

// CondFormats lists the sheet's rules in priority order.
func (s *SheetService) CondFormats(ctx context.Context, sheet int) []CFRuleDTO {
	out := []CFRuleDTO{}
	s.at(ctx).read(func(d *document.Document) {
		sh := sheetAt(d, sheet)
		if sh == nil {
			return
		}
		for _, r := range sh.Layout.CondFormats {
			dto := CFRuleDTO{Type: r.Type, Op: r.Op, Value1: r.Value1, Value2: r.Value2, Percent: r.Percent, Below: r.Below,
				Color: r.Format.Color, Fill: r.Format.Fill, Bold: r.Format.Bold, Italic: r.Format.Italic,
				Colors: append([]string{}, r.Colors...), StopIfTrue: r.StopIfTrue, Ranges: []RangeDTO{}}
			for _, rg := range r.Ranges {
				dto.Ranges = append(dto.Ranges, rangeDTO(rg))
			}
			out = append(out, dto)
		}
	})
	return out
}

func (s *SheetService) AddCondFormat(ctx context.Context, sheet int, ranges []RangeDTO, rule CFRuleDTO) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.AddCFRuleCommand(sheet, toModelRanges(ranges), rule.model())
	})
}

func (s *SheetService) DeleteCondFormat(ctx context.Context, sheet, index int) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.DeleteCFRuleCommand(sheet, index)
	})
}

// ClearCondFormats removes rules from ranges, or from the whole sheet
// when ranges is empty.
func (s *SheetService) ClearCondFormats(ctx context.Context, sheet int, ranges []RangeDTO) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.ClearCFCommand(sheet, toModelRanges(ranges))
	})
}

// ---- Tables ----

type TableDTO struct {
	Name   string   `json:"name"`
	Range  RangeDTO `json:"range"`
	Totals bool     `json:"totals"`
	Style  string   `json:"style"`
	Bands  bool     `json:"bands"`
}

func tableDTO(t model.Table) TableDTO {
	return TableDTO{Name: t.Name, Range: rangeDTO(t.Range), Totals: t.TotalsRow, Style: t.Style, Bands: !t.NoBands}
}

// TableStyleList lists the styles offered for tables.
func (s *SheetService) TableStyleList(ctx context.Context) []string { return TableStyles }

// TableRange suggests the range for 格式化為表格: the selection, or the
// data region around the active cell, and whether it has headers.
func (s *SheetService) TableRange(ctx context.Context, sheet int, sel RangeDTO) SortInfo {
	return s.SortInfo(ctx, sheet, sel)
}

// CreateTable makes a table and returns its name.
func (s *SheetService) CreateTable(ctx context.Context, sheet int, rg RangeDTO, hasHeaders bool, style string) (string, error) {
	var name string
	err := s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		cmd, n, err := d.CreateTableCommand(sheet, rg.model(), hasHeaders, style)
		name = n
		return cmd, err
	})
	return name, err
}

// TableAt returns the table containing a cell, or nil.
func (s *SheetService) TableAt(ctx context.Context, sheet, row, col int) *TableDTO {
	var out *TableDTO
	s.at(ctx).read(func(d *document.Document) {
		if sh := sheetAt(d, sheet); sh != nil {
			if t := sh.Layout.TableAt(row, col); t != nil {
				dto := tableDTO(*t)
				out = &dto
			}
		}
	})
	return out
}

func (s *SheetService) SetTableStyle(ctx context.Context, sheet int, name, style string, bands bool) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.SetTableStyleCommand(sheet, name, style, bands)
	})
}

func (s *SheetService) RenameTable(ctx context.Context, sheet int, old, name string) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.RenameTableCommand(sheet, old, name)
	})
}

func (s *SheetService) ToggleTableTotals(ctx context.Context, sheet int, name string) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.ToggleTotalsCommand(sheet, name)
	})
}

func (s *SheetService) ConvertTable(ctx context.Context, sheet int, name string) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.ConvertTableCommand(sheet, name)
	})
}

// parseDateText turns a typed date or time into its serial number text.
func parseDateText(s string) (string, bool) {
	v, code, ok := numfmt.Detect(s, time.Now())
	if !ok || !numfmt.IsDate(code) {
		return "", false
	}
	return numfmt.General(v), true
}
