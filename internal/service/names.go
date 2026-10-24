package service

import (
	"context"
	"errors"

	"ww-excel/internal/command"
	"ww-excel/internal/document"
)

type NameDTO struct {
	Name     string `json:"name"`
	RefersTo string `json:"refersTo"`
	// Scope is the sheet index, or -1 for the whole workbook.
	Scope   int    `json:"scope"`
	Comment string `json:"comment"`
}

// Names lists the defined names.
func (ds *DocumentService) Names(ctx context.Context) []NameDTO {
	out := []NameDTO{}
	ds.at(ctx).read(func(d *document.Document) {
		wb := d.Workbook()
		for _, n := range wb.Names {
			dto := NameDTO{Name: n.Name, RefersTo: "=" + n.Formula, Scope: -1, Comment: n.Comment}
			if n.Scope != nil {
				dto.Scope = wb.SheetIndex(n.Scope.Name)
			}
			out = append(out, dto)
		}
	})
	return out
}

// SetName adds a name, or changes the one called oldName in oldScope
// (oldName == "" adds).
func (ds *DocumentService) SetName(ctx context.Context, oldName string, oldScope int, n NameDTO) error {
	return ds.sheetChange(ctx, func(d *document.Document) (command.Command, error) {
		in := document.NameInput{Name: n.Name, Scope: n.Scope, RefersTo: n.RefersTo, Comment: n.Comment}
		if oldName == "" {
			return d.SetNameCommand(nil, in)
		}
		old := d.FindName(oldName, oldScope)
		if old == nil {
			return nil, errors.New("名稱不存在")
		}
		return d.SetNameCommand(old, in)
	})
}

func (ds *DocumentService) DeleteName(ctx context.Context, name string, scope int) error {
	return ds.sheetChange(ctx, func(d *document.Document) (command.Command, error) {
		n := d.FindName(name, scope)
		if n == nil {
			return nil, errors.New("名稱不存在")
		}
		return d.DeleteNameCommand(n)
	})
}

// TargetDTO is where name-box text points; OK is false when it is
// neither an address nor a name of a range.
type TargetDTO struct {
	OK    bool     `json:"ok"`
	Sheet int      `json:"sheet"`
	Range RangeDTO `json:"range"`
}

// Resolve interprets text typed in the name box on sheet.
func (ds *DocumentService) Resolve(ctx context.Context, sheet int, text string) TargetDTO {
	var out TargetDTO
	ds.at(ctx).read(func(d *document.Document) {
		if t, ok := d.Resolve(sheet, text); ok {
			out = TargetDTO{OK: true, Sheet: t.Sheet, Range: rangeDTO(t.Range)}
		}
	})
	return out
}
