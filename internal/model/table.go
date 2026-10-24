package model

import "strings"

// Table is an Excel table (表格, ListObject): a range with a header row,
// optional totals row, banded formatting and structured references.
type Table struct {
	Name      string
	Range     Range // header row through totals row
	TotalsRow bool
	// Style is the table style name, e.g. TableStyleMedium2.
	Style string
	// NoBands turns off banded rows.
	NoBands bool
}

// DataRows is the range of the table's data rows (R0 > R1 when the
// table has no data rows).
func (t Table) DataRows() Range {
	r := t.Range
	r.R0++
	if t.TotalsRow {
		r.R1--
	}
	return r
}

// Columns returns the header names (cell text of the header row).
func (t Table) Columns(sh *Sheet) []string {
	out := make([]string, t.Range.C1-t.Range.C0+1)
	for i := range out {
		out[i] = sh.Get(t.Range.R0, t.Range.C0+i).Display()
	}
	return out
}

// ColumnIndex finds a column by header name (case-insensitive), as an
// absolute column index.
func (t Table) ColumnIndex(sh *Sheet, name string) (int, bool) {
	for i, c := range t.Columns(sh) {
		if strings.EqualFold(c, name) {
			return t.Range.C0 + i, true
		}
	}
	return 0, false
}

// FindTable finds a table by name in any sheet.
func (wb *Workbook) FindTable(name string) (*Table, *Sheet) {
	for _, sh := range wb.Sheets {
		for i := range sh.Layout.Tables {
			if strings.EqualFold(sh.Layout.Tables[i].Name, name) {
				return &sh.Layout.Tables[i], sh
			}
		}
	}
	return nil, nil
}

// TableAt returns the table containing a cell, if any.
func (l *Layout) TableAt(row, col int) *Table {
	for i := range l.Tables {
		if l.Tables[i].Range.Contains(row, col) {
			return &l.Tables[i]
		}
	}
	return nil
}
