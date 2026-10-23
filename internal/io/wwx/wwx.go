// Package wwx is the native file format: gzip-compressed JSON holding
// every sheet's non-empty cells. It is meant for fast save/load of
// everything the model supports; xlsx (v0.4) is the interchange format.
package wwx

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"

	"ww-excel/internal/model"
)

const (
	formatID       = "ww-excel"
	currentVersion = 1
)

type fileDoc struct {
	Format  string      `json:"format"`
	Version int         `json:"version"`
	Sheets  []fileSheet `json:"sheets"`
}

type fileSheet struct {
	Name  string     `json:"name"`
	Cells []fileCell `json:"cells"`
}

// Exactly one of N or S is set.
type fileCell struct {
	R int      `json:"r"`
	C int      `json:"c"`
	N *float64 `json:"n,omitempty"`
	S *string  `json:"s,omitempty"`
}

func Write(w io.Writer, wb *model.Workbook) error {
	doc := fileDoc{Format: formatID, Version: currentVersion}
	all := model.Range{R0: 0, C0: 0, R1: model.MaxRows - 1, C1: model.MaxCols - 1}
	for _, sh := range wb.Sheets {
		fs := fileSheet{Name: sh.Name, Cells: []fileCell{}}
		sh.EachInRange(all, func(row, col int, c model.Cell) {
			fc := fileCell{R: row, C: col}
			switch c.Kind {
			case model.KindNumber:
				fc.N = &c.Num
			case model.KindString:
				fc.S = &c.Str
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
	for _, fs := range doc.Sheets {
		sh := model.NewSheet(fs.Name)
		for _, fc := range fs.Cells {
			if fc.R < 0 || fc.R >= model.MaxRows || fc.C < 0 || fc.C >= model.MaxCols {
				return nil, fmt.Errorf("sheet %q: cell (%d,%d) out of range", fs.Name, fc.R, fc.C)
			}
			switch {
			case fc.N != nil:
				sh.Set(fc.R, fc.C, model.NumberCell(*fc.N))
			case fc.S != nil:
				sh.Set(fc.R, fc.C, model.StringCell(*fc.S))
			}
		}
		wb.Sheets = append(wb.Sheets, sh)
	}
	return wb, nil
}
