package service

import (
	"context"
	"errors"
	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

type PivotInfoDTO struct {
	Pivot  model.Pivot         `json:"pivot"`
	Fields []string            `json:"fields"`
	Items  map[string][]string `json:"items"`
	Error  string              `json:"error"`
}
type PivotFilterDTO struct {
	Name   string           `json:"name"`
	Field  string           `json:"field"`
	Values []FilterValueDTO `json:"values"`
	Error  string           `json:"error"`
}

func pivotInfo(d *document.Document, sheet int, p model.Pivot) PivotInfoDTO {
	out := PivotInfoDTO{Pivot: p.Clone(), Fields: []string{}, Items: map[string][]string{}, Error: p.Unsupported}
	src, err := d.ResolvePivotSource(sheet, p.Source)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.Fields = src.Fields
	for _, f := range src.Fields {
		values, err := d.PivotItems(sheet, p, f)
		if err != nil {
			out.Error = err.Error()
			break
		}
		out.Items[f] = values
	}
	return out
}
func (s *SheetService) Pivots(ctx context.Context, sheet int) []PivotInfoDTO {
	out := []PivotInfoDTO{}
	s.at(ctx).read(func(d *document.Document) {
		if sh := sheetAt(d, sheet); sh != nil {
			for _, p := range sh.Layout.Pivots {
				out = append(out, pivotInfo(d, sheet, p))
			}
		}
	})
	return out
}
func (s *SheetService) UpdatePivot(ctx context.Context, sheet int, p model.Pivot) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) { return d.PivotCommand(sheet, p) })
}
func (s *SheetService) RefreshPivot(ctx context.Context, sheet int, name string) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) { return d.RefreshPivotCommand(sheet, name) })
}
func (s *SheetService) PivotFilter(ctx context.Context, sheet, row, col int) *PivotFilterDTO {
	var out *PivotFilterDTO
	s.at(ctx).read(func(d *document.Document) {
		sh := sheetAt(d, sheet)
		if sh == nil {
			return
		}
		for _, p := range sh.Layout.Pivots {
			for _, b := range p.Buttons() {
				if b.Row != row || b.Col != col {
					continue
				}
				out = &PivotFilterDTO{Name: p.Name, Field: b.Field, Values: []FilterValueDTO{}, Error: p.Unsupported}
				values, err := d.PivotItems(sheet, p, b.Field)
				if err != nil {
					out.Error = err.Error()
					return
				}
				allowed, on := p.Filters[b.Field]
				for _, v := range values {
					shown := !on
					for _, a := range allowed {
						if a == v {
							shown = true
						}
					}
					out.Values = append(out.Values, FilterValueDTO{Text: v, Shown: shown})
				}
				return
			}
		}
	})
	return out
}
func (s *SheetService) SetPivotFilter(ctx context.Context, sheet int, name, field string, shown []string, clear bool) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		for _, p := range d.Workbook().Sheets[sheet].Layout.Pivots {
			if p.Name == name {
				p = p.Clone()
				if p.Filters == nil {
					p.Filters = map[string][]string{}
				}
				if clear {
					delete(p.Filters, field)
				} else {
					p.Filters[field] = append([]string{}, shown...)
				}
				return d.PivotCommand(sheet, p)
			}
		}
		return nil, errors.New("樞紐分析表不存在")
	})
}
