package model

import "slices"

// Pivot stores an editable worksheet-source pivot independently of AutoFilter.
// Imported unsupported features keep their cached cells until explicitly supported.
type Pivot struct {
	Name        string
	Source      string
	Range       Range
	Rows        []string
	Columns     []string
	Pages       []string
	Values      []PivotValue
	Filters     map[string][]string
	NoBands     bool
	Subtotals   bool
	RowTotals   bool
	ColTotals   bool
	Style       string
	Appearance  *TableAppearance `json:",omitempty"`
	HeaderStyle uint32
	TotalStyle  uint32
	RowStyle    uint32
	Unsupported string `json:",omitempty"`
}
type PivotValue struct {
	Field    string
	Function string
	Caption  string
	Format   string
}

func (p Pivot) Clone() Pivot {
	p.Rows = slices.Clone(p.Rows)
	p.Columns = slices.Clone(p.Columns)
	p.Pages = slices.Clone(p.Pages)
	p.Values = slices.Clone(p.Values)
	if p.Filters != nil {
		f := make(map[string][]string, len(p.Filters))
		for k, v := range p.Filters {
			f[k] = slices.Clone(v)
		}
		p.Filters = f
	}
	return p
}
func (l *Layout) PivotAt(row, col int) *Pivot {
	for i := range l.Pivots {
		if l.Pivots[i].Range.Contains(row, col) {
			return &l.Pivots[i]
		}
	}
	return nil
}

type PivotButton struct {
	Name     string `json:"name"`
	Field    string `json:"field"`
	Row      int    `json:"row"`
	Col      int    `json:"col"`
	Filtered bool   `json:"filtered"`
}

func (p Pivot) Buttons() []PivotButton {
	out := []PivotButton{}
	add := func(row, col int, field string) {
		_, filtered := p.Filters[field]
		out = append(out, PivotButton{Name: p.Name, Field: field, Row: row, Col: col, Filtered: filtered})
	}
	base := p.Range.R0
	if len(p.Pages) > 0 {
		for i, f := range p.Pages {
			add(base+i, p.Range.C0+1, f)
		}
		base += len(p.Pages) + 1
	}
	for i, f := range p.Rows {
		add(base, p.Range.C0+i, f)
	}
	lastColumn := p.Range.C1
	if p.RowTotals {
		lastColumn -= max(1, len(p.Values))
	}
	for i, f := range p.Columns {
		for col := p.Range.C0 + max(1, len(p.Rows)); col <= lastColumn; col += max(1, len(p.Values)) {
			add(base+i, col, f)
		}
	}
	return out
}
