// Package csvfile reads and writes a single sheet as CSV, in the dialect
// Excel produces and expects: UTF-8 with BOM, comma separated, CRLF.
package csvfile

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"

	"ww-excel/internal/model"
)

var bom = []byte{0xEF, 0xBB, 0xBF}

// Result is a loaded sheet; Truncated reports data beyond the grid limits
// that was dropped.
type Result struct {
	Sheet     *model.Sheet
	Truncated bool
}

func Read(r io.Reader, sheetName string) (Result, error) {
	br := bufio.NewReader(r)
	if head, err := br.Peek(3); err == nil && string(head) == string(bom) {
		_, _ = br.Discard(3)
	}

	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.ReuseRecord = true

	res := Result{Sheet: model.NewSheet(sheetName)}
	for row := 0; ; row++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return res, nil
		}
		if err != nil {
			return res, fmt.Errorf("csv row %d: %w", row+1, err)
		}
		if row >= model.MaxRows {
			res.Truncated = true
			return res, nil
		}
		if len(rec) > model.MaxCols {
			res.Truncated = true
			rec = rec[:model.MaxCols]
		}
		for col, field := range rec {
			if cell := model.ParseValue(field); cell.Kind != model.KindEmpty {
				res.Sheet.Set(row, col, cell)
			}
		}
	}
}

// Write emits every row up to the last non-empty one, each padded to the
// width of the data, like Excel's "CSV UTF-8" export.
func Write(w io.Writer, s *model.Sheet) error {
	type cell struct {
		row, col int
		text     string
	}
	var cells []cell
	rows, cols := 0, 0
	s.EachInRange(model.Range{R0: 0, C0: 0, R1: model.MaxRows - 1, C1: model.MaxCols - 1},
		func(row, col int, c model.Cell) {
			cells = append(cells, cell{row, col, c.Display()})
			rows, cols = max(rows, row+1), max(cols, col+1)
		})

	bw := bufio.NewWriter(w)
	if _, err := bw.Write(bom); err != nil {
		return err
	}
	cw := csv.NewWriter(bw)
	cw.UseCRLF = true

	rec := make([]string, cols)
	next := 0
	for row := 0; row < rows; row++ {
		clear(rec)
		for next < len(cells) && cells[next].row == row {
			rec[cells[next].col] = cells[next].text
			next++
		}
		if cols == 1 && rec[0] == "" {
			// A bare empty line would be skipped by CSV readers and shift
			// every following row, so write an explicit empty field.
			cw.Flush()
			if _, err := bw.WriteString("\"\"\r\n"); err != nil {
				return err
			}
			continue
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	return bw.Flush()
}
