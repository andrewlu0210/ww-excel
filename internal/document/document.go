// Package document ties a workbook to its file and edit history.
// It is not safe for concurrent use; callers serialize access.
package document

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ww-excel/internal/command"
	"ww-excel/internal/formula"
	"ww-excel/internal/formula/ast"
	"ww-excel/internal/io/csvfile"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

type Format string

const (
	FormatNative Format = "wwx"
	FormatCSV    Format = "csv"
)

// UntitledName is shown for documents that were never saved.
const UntitledName = "未命名"

func FormatForPath(path string) Format {
	if strings.EqualFold(filepath.Ext(path), ".csv") {
		return FormatCSV
	}
	return FormatNative
}

type Document struct {
	wb      *model.Workbook
	history *command.History
	calc    *formula.Engine
	path    string
	// version increases on every change so the frontend can discard
	// block fetches that raced with an edit.
	version uint64
	// Warning from the last load, e.g. truncated CSV.
	warning string
}

func New() *Document {
	return FromWorkbook(model.NewWorkbook(), "")
}

// FromWorkbook wraps a workbook and calculates all its formulas.
func FromWorkbook(wb *model.Workbook, path string) *Document {
	d := &Document{wb: wb, history: command.NewHistory(0), calc: formula.New(wb), path: path}
	d.calc.Rebuild()
	return d
}

// Load opens a .wwx or .csv file.
func Load(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	switch FormatForPath(path) {
	case FormatCSV:
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		res, err := csvfile.Read(f, name)
		if err != nil {
			return nil, err
		}
		// Excel opens "=..." fields as formulas; invalid ones stay text.
		calc := formula.New(&model.Workbook{Sheets: []*model.Sheet{res.Sheet}})
		res.Sheet.EachInRange(model.Range{R1: model.MaxRows - 1, C1: model.MaxCols - 1}, func(row, col int, c model.Cell) {
			if c.F == nil {
				return
			}
			if f, err := calc.Compile(c.F.Text, 0); err == nil {
				res.Sheet.Set(row, col, model.Cell{F: f})
			} else {
				res.Sheet.Set(row, col, model.StringCell("="+c.F.Text))
			}
		})
		d := FromWorkbook(&model.Workbook{Sheets: []*model.Sheet{res.Sheet}}, path)
		if res.Truncated {
			d.warning = "檔案超出工作表上限（1,048,576 列 × 16,384 欄），超出的資料未載入。"
		}
		return d, nil
	default:
		wb, err := wwx.Read(f)
		if err != nil {
			return nil, err
		}
		return FromWorkbook(wb, path), nil
	}
}

// SaveAs writes the document to path in the format implied by its
// extension. The file is written to a temp file first and renamed, so a
// failed save never leaves a half-written file behind.
func (d *Document) SaveAs(path string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	switch FormatForPath(path) {
	case FormatCSV:
		err = csvfile.Write(tmp, d.wb.Sheets[0])
	default:
		err = wwx.Write(tmp, d.wb)
	}
	if err == nil {
		// CreateTemp uses 0600; keep the existing file's mode or use 0644.
		mode := os.FileMode(0o644)
		if fi, statErr := os.Stat(path); statErr == nil {
			mode = fi.Mode().Perm()
		}
		err = tmp.Chmod(mode)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("save %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}

	d.path = path
	d.history.MarkSaved()
	return nil
}

func (d *Document) Workbook() *model.Workbook { return d.wb }
func (d *Document) History() *command.History { return d.history }
func (d *Document) Path() string              { return d.path }
func (d *Document) Version() uint64           { return d.version }
func (d *Document) Dirty() bool               { return d.history.Dirty() }

// TakeWarning returns and clears the load warning.
func (d *Document) TakeWarning() string {
	w := d.warning
	d.warning = ""
	return w
}

// Name is the file name shown in the title bar.
func (d *Document) Name() string {
	if d.path == "" {
		return UntitledName
	}
	return filepath.Base(d.path)
}

// Change is the result of an edit, undo or redo: the edited cells plus
// the formula cells recalculated because of it.
type Change struct {
	command.Effect
	Recalc []formula.Key
}

// Exec runs cmd; a nil cmd (a no-op edit) reports false.
func (d *Document) Exec(cmd command.Command) (Change, bool) {
	if cmd == nil {
		return Change{}, false
	}
	d.version++
	return d.recalc(d.history.Exec(d.wb, cmd)), true
}

func (d *Document) Undo() (Change, bool) {
	eff, ok := d.history.Undo(d.wb)
	if !ok {
		return Change{}, false
	}
	d.version++
	return d.recalc(eff), true
}

func (d *Document) Redo() (Change, bool) {
	eff, ok := d.history.Redo(d.wb)
	if !ok {
		return Change{}, false
	}
	d.version++
	return d.recalc(eff), true
}

func (d *Document) recalc(eff command.Effect) Change {
	keys := make([]formula.Key, len(eff.Cells))
	for i, c := range eff.Cells {
		keys[i] = formula.Key{Sheet: eff.Sheet, Row: c.Row, Col: c.Col}
	}
	return Change{Effect: eff, Recalc: d.calc.Update(keys)}
}

// InputCell turns typed text into a cell, compiling formulas. A formula
// the user cannot enter returns an error with a message for the user.
func (d *Document) InputCell(sheet int, text string) (model.Cell, error) {
	cell := model.ParseInput(text)
	if cell.F == nil {
		return cell, nil
	}
	f, err := d.calc.Compile(cell.F.Text, sheet)
	if err != nil {
		return model.Cell{}, err
	}
	return model.Cell{F: f}, nil
}

// FillCells returns a cell factory for ⌘Enter: a formula typed at the
// active cell is adjusted for every other cell, like Excel's fill.
func (d *Document) FillCells(sheet int, text string, activeRow, activeCol int) (func(row, col int) model.Cell, error) {
	cell, err := d.InputCell(sheet, text)
	if err != nil {
		return nil, err
	}
	if cell.F == nil {
		return func(int, int) model.Cell { return cell }, nil
	}
	node := d.calc.Parsed(cell.F, sheet)
	return func(row, col int) model.Cell {
		if row == activeRow && col == activeCol {
			return cell
		}
		moved := ast.Translate(node, row-activeRow, col-activeCol)
		return model.Cell{F: d.calc.CompileNode(moved, sheet)}
	}, nil
}

// Circular returns the address of the first cell in a circular
// reference ("A1" or "Sheet2!A1"), or "".
func (d *Document) Circular() string {
	keys := d.calc.Circular()
	if len(keys) == 0 {
		return ""
	}
	k := keys[0]
	addr := ast.FormatArea(ast.CellAt(k.Row, k.Col, false, false))
	if k.Sheet != 0 {
		addr = ast.QuoteSheet(d.wb.Sheets[k.Sheet].Name) + "!" + addr
	}
	return addr
}
