// Package document ties a workbook to its file and edit history.
// It is not safe for concurrent use; callers serialize access.
package document

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"ww-excel/internal/command"
	"ww-excel/internal/formula"
	"ww-excel/internal/io/csvfile"
	"ww-excel/internal/io/wwx"
	"ww-excel/internal/io/xlsxfile"
	"ww-excel/internal/model"
)

type Format string

const (
	FormatNative Format = "wwx"
	FormatCSV    Format = "csv"
	FormatXLSX   Format = "xlsx"
)

// UntitledName is shown for documents that were never saved.
const UntitledName = "未命名"

func FormatForPath(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return FormatCSV
	case ".xlsx", ".xlsm":
		return FormatXLSX
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

	// The xlsx file the workbook came from (or was last saved to), kept
	// for in-place saves; with what changed since.
	xlsx         *excelize.File
	changedCells map[*model.Sheet]map[[2]int]bool
	layoutBase   map[*model.Sheet]model.Layout
	layoutDirty  map[*model.Sheet]bool
	// Row/column insertions and deletions since the last save, replayed
	// on the source file before writing changes.
	shifts map[*model.Sheet][]xlsxfile.Shift
	// Defined names changed since the last save.
	namesDirty bool

	// Now is injectable for tests (date input detection).
	Now func() time.Time
}

func New() *Document {
	return FromWorkbook(model.NewWorkbook(), "")
}

// FromWorkbook wraps a workbook and calculates all its formulas.
func FromWorkbook(wb *model.Workbook, path string) *Document {
	d := &Document{wb: wb, history: command.NewHistory(0), calc: formula.New(wb), path: path, Now: time.Now}
	d.calc.Rebuild()
	d.resetChanges()
	return d
}

// Load opens a .xlsx, .wwx or .csv file.
func Load(path string) (*Document, error) {
	switch FormatForPath(path) {
	case FormatXLSX:
		wb, src, err := xlsxfile.Read(path)
		if err != nil {
			return nil, err
		}
		d := FromWorkbook(wb, path)
		d.xlsx = src
		return d, nil
	case FormatCSV:
		return loadCSV(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	wb, err := wwx.Read(f)
	if err != nil {
		return nil, err
	}
	return FromWorkbook(wb, path), nil
}

func loadCSV(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	res, err := csvfile.Read(f, name)
	if err != nil {
		return nil, err
	}
	wb := model.NewWorkbook()
	wb.Sheets = []*model.Sheet{res.Sheet}
	// Excel opens "=..." fields as formulas; invalid ones stay text.
	calc := formula.New(wb)
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
	d := FromWorkbook(wb, path)
	if res.Truncated {
		d.warning = "檔案超出工作表上限（1,048,576 列 × 16,384 欄），超出的資料未載入。"
	}
	return d, nil
}

// Close releases the xlsx source file, if any.
func (d *Document) Close() {
	if d.xlsx != nil {
		d.xlsx.Close()
		d.xlsx = nil
	}
}

// SaveAs writes the document to path in the format implied by its
// extension. Files are written to a temp file and renamed, so a failed
// save never leaves a half-written file behind.
func (d *Document) SaveAs(path string) error {
	var err error
	switch FormatForPath(path) {
	case FormatXLSX:
		err = d.saveXLSX(path)
	case FormatCSV:
		err = writeAtomic(path, func(f *os.File) error { return csvfile.Write(f, d.wb.Sheets[0]) })
	default:
		err = writeAtomic(path, func(f *os.File) error { return wwx.Write(f, d.wb) })
	}
	if err != nil {
		return fmt.Errorf("save %s: %w", filepath.Base(path), err)
	}
	d.path = path
	d.history.MarkSaved()
	return nil
}

func (d *Document) saveXLSX(path string) error {
	ch := xlsxfile.Changes{Cells: d.changedCells, OldLayouts: map[*model.Sheet]model.Layout{}, Shifts: d.shifts, Names: d.namesDirty}
	for sh := range d.layoutDirty {
		ch.OldLayouts[sh] = d.layoutBase[sh]
	}
	f, err := xlsxfile.Save(path, d.wb, d.xlsx, ch)
	if err != nil {
		return err
	}
	if d.xlsx != nil && d.xlsx != f {
		d.xlsx.Close()
	}
	d.xlsx = f
	d.resetChanges()
	return nil
}

// resetChanges marks the current state as the state of the xlsx source.
func (d *Document) resetChanges() {
	d.changedCells = map[*model.Sheet]map[[2]int]bool{}
	d.layoutDirty = map[*model.Sheet]bool{}
	d.shifts = map[*model.Sheet][]xlsxfile.Shift{}
	d.namesDirty = false
	d.layoutBase = map[*model.Sheet]model.Layout{}
	for _, sh := range d.wb.Sheets {
		d.layoutBase[sh] = sh.Layout.Clone()
	}
}

func writeAtomic(path string, write func(*os.File) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	err = write(tmp)
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
		return err
	}
	return os.Rename(tmp.Name(), path)
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
	return d.apply(d.history.Exec(d.wb, cmd)), true
}

func (d *Document) Undo() (Change, bool) {
	eff, ok := d.history.Undo(d.wb)
	if !ok {
		return Change{}, false
	}
	d.version++
	return d.apply(eff), true
}

func (d *Document) Redo() (Change, bool) {
	eff, ok := d.history.Redo(d.wb)
	if !ok {
		return Change{}, false
	}
	d.version++
	return d.apply(eff), true
}

// apply records what changed (for in-place xlsx saves) and recalculates.
func (d *Document) apply(eff command.Effect) Change {
	mark := func(sheet int, cells []command.CellRef) {
		if sheet < 0 || sheet >= len(d.wb.Sheets) {
			return
		}
		sh := d.wb.Sheets[sheet]
		set := d.changedCells[sh]
		if set == nil {
			set = map[[2]int]bool{}
			d.changedCells[sh] = set
		}
		for _, c := range cells {
			set[[2]int{c.Row, c.Col}] = true
		}
	}
	if eff.Shift == nil {
		mark(eff.Sheet, eff.Cells)
	}
	for sheet, cells := range eff.Others {
		mark(sheet, cells)
	}
	if eff.Shift != nil && eff.Sheet < len(d.wb.Sheets) {
		// Earlier edits moved with the cells.
		sh := d.wb.Sheets[eff.Sheet]
		d.changedCells[sh] = shiftPositions(d.changedCells[sh], *eff.Shift)
		mark(eff.Sheet, eff.Cells)
	}
	if eff.LayoutChanged && eff.Sheet < len(d.wb.Sheets) {
		sh := d.wb.Sheets[eff.Sheet]
		if _, ok := d.layoutBase[sh]; ok {
			d.layoutDirty[sh] = true
		}
	}

	if eff.Shift != nil && eff.Sheet < len(d.wb.Sheets) {
		d.recordShift(d.wb.Sheets[eff.Sheet], *eff.Shift)
	}
	if eff.Names {
		d.namesDirty = true
	}
	if eff.Structure || eff.Shift != nil || eff.Names {
		// Sheets or positions changed: resolve every reference again.
		return Change{Effect: eff, Recalc: d.calc.Rebuild()}
	}
	keys := make([]formula.Key, 0, len(eff.Cells))
	for _, c := range eff.Cells {
		keys = append(keys, formula.Key{Sheet: eff.Sheet, Row: c.Row, Col: c.Col})
	}
	for sheet, cells := range eff.Others {
		for _, c := range cells {
			keys = append(keys, formula.Key{Sheet: sheet, Row: c.Row, Col: c.Col})
		}
	}
	return Change{Effect: eff, Recalc: d.calc.Update(keys)}
}

// Circular returns the address of the first cell in a circular
// reference ("A1" or "Sheet2!A1"), or "".
func (d *Document) Circular() string {
	keys := d.calc.Circular()
	if len(keys) == 0 {
		return ""
	}
	k := keys[0]
	return d.address(k.Sheet, k.Row, k.Col, k.Sheet != 0)
}

// recordShift remembers a structural edit for the next in-place save and
// keeps the saved-layout baseline aligned with the file after replay.
func (d *Document) recordShift(sh *model.Sheet, s command.Shift) {
	d.shifts[sh] = append(d.shifts[sh], xlsxfile.Shift{Rows: s.Rows, At: s.At, Count: s.Count})
	if base, ok := d.layoutBase[sh]; ok {
		base = base.Clone()
		base.Shift(s.Rows, s.At, s.Count)
		d.layoutBase[sh] = base
	}
}

func shiftPositions(set map[[2]int]bool, s command.Shift) map[[2]int]bool {
	out := make(map[[2]int]bool, len(set))
	for pos := range set {
		i := pos[1]
		if s.Rows {
			i = pos[0]
		}
		ni, keep := model.ShiftIndex(i, s.At, s.Count)
		if !keep {
			continue
		}
		if s.Rows {
			out[[2]int{ni, pos[1]}] = true
		} else {
			out[[2]int{pos[0], ni}] = true
		}
	}
	return out
}
