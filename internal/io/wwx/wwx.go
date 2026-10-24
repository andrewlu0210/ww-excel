// Package wwx is the native file format: gzip-compressed JSON holding
// every sheet's non-empty cells. It is meant for fast save/load of
// everything the model supports; xlsx (v0.4) is the interchange format.
package wwx

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"ww-excel/internal/model"
)

const (
	formatID = "ww-excel"
	// Version 2 adds styles and sheet layout; version 1 files still load.
	currentVersion = 2
)

type fileDoc struct {
	Format  string        `json:"format"`
	Version int           `json:"version"`
	Styles  []model.Style `json:"styles,omitempty"`
	Sheets  []fileSheet   `json:"sheets"`
}

type fileSheet struct {
	Name   string      `json:"name"`
	Layout *fileLayout `json:"layout,omitempty"`
	Cells  []fileCell  `json:"cells"`
}

type fileSize struct {
	I      int     `json:"i"`
	Px     float64 `json:"px"`
	Hidden bool    `json:"hidden,omitempty"`
}

type fileStyleRef struct {
	I int    `json:"i"`
	S uint32 `json:"s"`
}

type fileLayout struct {
	DefaultRow float64        `json:"defaultRow"`
	DefaultCol float64        `json:"defaultCol"`
	Rows       []fileSize     `json:"rows,omitempty"`
	Cols       []fileSize     `json:"cols,omitempty"`
	RowStyles  []fileStyleRef `json:"rowStyles,omitempty"`
	ColStyles  []fileStyleRef `json:"colStyles,omitempty"`
	Merges     [][4]int       `json:"merges,omitempty"`
	FreezeRows int            `json:"freezeRows,omitempty"`
	FreezeCols int            `json:"freezeCols,omitempty"`
	Filter     *fileFilter    `json:"filter,omitempty"`
}

type fileFilter struct {
	Range [4]int           `json:"range"`
	Cols  map[int][]string `json:"cols,omitempty"`
}

// A cell has at most one value field (n, s, b or e). F is the formula
// text without "="; the value then is its cached result.
type fileCell struct {
	R  int      `json:"r"`
	C  int      `json:"c"`
	N  *float64 `json:"n,omitempty"`
	S  *string  `json:"s,omitempty"`
	B  *bool    `json:"b,omitempty"`
	E  string   `json:"e,omitempty"`
	F  string   `json:"f,omitempty"`
	St uint32   `json:"st,omitempty"`
}

func Write(w io.Writer, wb *model.Workbook) error {
	doc := fileDoc{Format: formatID, Version: currentVersion, Styles: wb.StyleTable().Since(0)}
	all := model.Range{R0: 0, C0: 0, R1: model.MaxRows - 1, C1: model.MaxCols - 1}
	for _, sh := range wb.Sheets {
		fs := fileSheet{Name: sh.Name, Layout: writeLayout(sh.Layout), Cells: []fileCell{}}
		sh.EachInRange(all, func(row, col int, c model.Cell) {
			fc := fileCell{R: row, C: col, St: c.S}
			switch c.Kind {
			case model.KindNumber:
				fc.N = &c.Num
			case model.KindString:
				fc.S = &c.Str
			case model.KindBool:
				b := c.Num != 0
				fc.B = &b
			case model.KindError:
				fc.E = c.Str
			}
			if c.F != nil {
				fc.F = c.F.Text
			}
			fs.Cells = append(fs.Cells, fc)
		})
		doc.Sheets = append(doc.Sheets, fs)
	}

	zw := gzip.NewWriter(w)
	if err := json.NewEncoder(zw).Encode(doc); err != nil {
		return err
	}
	return zw.Close()
}

func Read(r io.Reader) (*model.Workbook, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("not a .wwx file: %w", err)
	}
	defer zr.Close()

	var doc fileDoc
	if err := json.NewDecoder(zr).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode .wwx: %w", err)
	}
	if doc.Format != formatID {
		return nil, fmt.Errorf("not a .wwx file (format %q)", doc.Format)
	}
	if doc.Version > currentVersion {
		return nil, fmt.Errorf(".wwx version %d is newer than supported %d", doc.Version, currentVersion)
	}
	if len(doc.Sheets) == 0 {
		return nil, fmt.Errorf(".wwx has no sheets")
	}

	wb := &model.Workbook{}
	if len(doc.Styles) > 0 {
		wb.Styles = model.NewStyleTable(doc.Styles[0])
		for i, st := range doc.Styles[1:] {
			if id := wb.Styles.Intern(st); int(id) != i+1 {
				return nil, fmt.Errorf(".wwx has duplicate styles")
			}
		}
	}
	nstyles := uint32(wb.StyleTable().Len())
	for _, fs := range doc.Sheets {
		sh := model.NewSheet(fs.Name)
		if fs.Layout != nil {
			sh.Layout = readLayout(fs.Layout)
		}
		for _, fc := range fs.Cells {
			if fc.R < 0 || fc.R >= model.MaxRows || fc.C < 0 || fc.C >= model.MaxCols {
				return nil, fmt.Errorf("sheet %q: cell (%d,%d) out of range", fs.Name, fc.R, fc.C)
			}
			var cell model.Cell
			switch {
			case fc.N != nil:
				cell = model.NumberCell(*fc.N)
			case fc.S != nil:
				cell = model.StringCell(*fc.S)
			case fc.B != nil:
				cell = model.BoolCell(*fc.B)
			case fc.E != "":
				cell = model.ErrorCell(fc.E)
			}
			if fc.F != "" {
				cell.F = &model.Formula{Text: fc.F}
			}
			if fc.St >= nstyles {
				return nil, fmt.Errorf("sheet %q: unknown style %d", fs.Name, fc.St)
			}
			cell.S = fc.St
			sh.Set(fc.R, fc.C, cell)
		}
		wb.Sheets = append(wb.Sheets, sh)
	}
	return wb, nil
}

func writeLayout(l model.Layout) *fileLayout {
	fl := &fileLayout{DefaultRow: l.DefaultRow, DefaultCol: l.DefaultCol, FreezeRows: l.FreezeRows, FreezeCols: l.FreezeCols}
	if f := l.Filter; f != nil {
		fl.Filter = &fileFilter{Range: [4]int{f.Range.R0, f.Range.C0, f.Range.R1, f.Range.C1}, Cols: f.Cols}
	}
	for i, sz := range l.Rows {
		fl.Rows = append(fl.Rows, fileSize{I: i, Px: sz.Px, Hidden: sz.Hidden})
	}
	for i, sz := range l.Cols {
		fl.Cols = append(fl.Cols, fileSize{I: i, Px: sz.Px, Hidden: sz.Hidden})
	}
	for i, st := range l.RowStyles {
		fl.RowStyles = append(fl.RowStyles, fileStyleRef{I: i, S: st})
	}
	for i, st := range l.ColStyles {
		fl.ColStyles = append(fl.ColStyles, fileStyleRef{I: i, S: st})
	}
	for _, m := range l.Merges {
		fl.Merges = append(fl.Merges, [4]int{m.R0, m.C0, m.R1, m.C1})
	}
	// Map iteration order is random; sort for stable files.
	sortSizes(fl.Rows)
	sortSizes(fl.Cols)
	sortRefs(fl.RowStyles)
	sortRefs(fl.ColStyles)
	return fl
}

func readLayout(fl *fileLayout) model.Layout {
	l := model.NewLayout()
	if fl.DefaultRow > 0 {
		l.DefaultRow = fl.DefaultRow
	}
	if fl.DefaultCol > 0 {
		l.DefaultCol = fl.DefaultCol
	}
	for _, s := range fl.Rows {
		l.Rows[s.I] = model.Size{Px: s.Px, Hidden: s.Hidden}
	}
	for _, s := range fl.Cols {
		l.Cols[s.I] = model.Size{Px: s.Px, Hidden: s.Hidden}
	}
	for _, r := range fl.RowStyles {
		l.RowStyles[r.I] = r.S
	}
	for _, r := range fl.ColStyles {
		l.ColStyles[r.I] = r.S
	}
	for _, m := range fl.Merges {
		l.Merges = append(l.Merges, model.Range{R0: m[0], C0: m[1], R1: m[2], C1: m[3]})
	}
	l.FreezeRows, l.FreezeCols = max(fl.FreezeRows, 0), max(fl.FreezeCols, 0)
	if f := fl.Filter; f != nil {
		r := f.Range
		l.Filter = &model.AutoFilter{Range: model.Range{R0: r[0], C0: r[1], R1: r[2], C1: r[3]}.Clamp(), Cols: f.Cols}
		if l.Filter.Cols == nil {
			l.Filter.Cols = map[int][]string{}
		}
	}
	return l
}

func sortSizes(s []fileSize) { sort.Slice(s, func(i, j int) bool { return s[i].I < s[j].I }) }
func sortRefs(s []fileStyleRef) {
	sort.Slice(s, func(i, j int) bool { return s[i].I < s[j].I })
}
