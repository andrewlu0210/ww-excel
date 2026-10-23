// Package document ties a workbook to its file and edit history.
// It is not safe for concurrent use; callers serialize access.
package document

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ww-excel/internal/command"
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

func FromWorkbook(wb *model.Workbook, path string) *Document {
	return &Document{wb: wb, history: command.NewHistory(0), path: path}
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

// Exec runs cmd; a nil cmd (a no-op edit) reports false.
func (d *Document) Exec(cmd command.Command) (command.Effect, bool) {
	if cmd == nil {
		return command.Effect{}, false
	}
	d.version++
	return d.history.Exec(d.wb, cmd), true
}

func (d *Document) Undo() (command.Effect, bool) {
	eff, ok := d.history.Undo(d.wb)
	if ok {
		d.version++
	}
	return eff, ok
}

func (d *Document) Redo() (command.Effect, bool) {
	eff, ok := d.history.Redo(d.wb)
	if ok {
		d.version++
	}
	return eff, ok
}
