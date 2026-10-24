package service

import (
	"context"
	"path/filepath"

	"ww-excel/internal/command"
	"ww-excel/internal/document"
)

type SaveChoice int

const (
	ChoiceSave SaveChoice = iota
	ChoiceDiscard
	ChoiceCancel
)

// Dialogs abstracts native dialogs so the service can be tested.
type Dialogs interface {
	// AskSave asks whether to save changes before discarding them.
	AskSave(docName string) SaveChoice
	// PickOpen returns "" when cancelled.
	PickOpen() (string, error)
	// PickSave returns "" when cancelled.
	PickSave(suggestedName string) (string, error)
	ShowError(title, message string)
	ShowWarning(title, message string)
	// Confirm asks an OK/Cancel question; true means OK.
	Confirm(title, message, ok string) bool
}

type DocumentService struct {
	src     Sessions
	dialogs Dialogs

	// OpenWindow shows a document in a new window. When nil (tests),
	// New and Open replace the calling window's document.
	OpenWindow func(doc *document.Document)
	// FocusPath brings the window showing path to the front, reporting
	// whether there was one.
	FocusPath func(path string) bool
	// Opened is called after a file is opened or saved (recent files).
	Opened func(path string)
}

func NewDocumentService(src Sessions, dialogs Dialogs) *DocumentService {
	return &DocumentService{src: src, dialogs: dialogs}
}

func (ds *DocumentService) at(ctx context.Context) *Session { return ds.src.For(ctx) }

func (ds *DocumentService) State(ctx context.Context) DocState {
	return ds.at(ctx).State()
}

// New opens an empty workbook in a new window.
func (ds *DocumentService) New(ctx context.Context) bool {
	if ds.OpenWindow != nil {
		ds.OpenWindow(document.New())
		return true
	}
	if !ds.confirmDiscard(ctx) {
		return false
	}
	ds.at(ctx).replace(document.New())
	return true
}

// Open asks for a file and opens it.
func (ds *DocumentService) Open(ctx context.Context) bool {
	if ds.OpenWindow == nil && !ds.confirmDiscard(ctx) {
		return false
	}
	path, err := ds.dialogs.PickOpen()
	if err != nil {
		ds.dialogs.ShowError("無法開啟檔案", err.Error())
		return false
	}
	if path == "" {
		return false
	}
	return ds.OpenFile(ctx, path)
}

// OpenFile opens path like Excel does: in the window already showing it,
// in the calling window when that holds an untouched new workbook, or
// in a new window.
func (ds *DocumentService) OpenFile(ctx context.Context, path string) bool {
	if ds.OpenWindow == nil {
		return ds.OpenPath(ctx, path) == nil
	}
	if ds.FocusPath != nil && ds.FocusPath(path) {
		return true
	}
	if s := ds.at(ctx); s != nil && s.Pristine() {
		return ds.OpenPath(ctx, path) == nil
	}
	doc, err := ds.load(path)
	if err != nil {
		return false
	}
	ds.OpenWindow(doc)
	return true
}

func (ds *DocumentService) load(path string) (*document.Document, error) {
	doc, err := document.Load(path)
	if err != nil {
		ds.dialogs.ShowError("無法開啟「"+filepath.Base(path)+"」", err.Error())
		return nil, err
	}
	if ds.Opened != nil {
		ds.Opened(path)
	}
	if w := doc.TakeWarning(); w != "" {
		go ds.dialogs.ShowWarning("「"+filepath.Base(path)+"」未完整載入", w)
	}
	return doc, nil
}

// OpenPath loads path into the calling window without asking about
// unsaved changes. Errors are also shown to the user.
func (ds *DocumentService) OpenPath(ctx context.Context, path string) error {
	doc, err := ds.load(path)
	if err != nil {
		return err
	}
	ds.at(ctx).replace(doc)
	return nil
}

// Save writes to the current path, asking for one if untitled.
func (ds *DocumentService) Save(ctx context.Context) bool {
	var path string
	ds.at(ctx).read(func(d *document.Document) { path = d.Path() })
	if path == "" {
		return ds.SaveAs(ctx)
	}
	return ds.saveTo(ctx, path)
}

func (ds *DocumentService) SaveAs(ctx context.Context) bool {
	var name string
	ds.at(ctx).read(func(d *document.Document) { name = d.Name() })
	if name == document.UntitledName {
		name += ".xlsx"
	}
	path, err := ds.dialogs.PickSave(name)
	if err != nil {
		ds.dialogs.ShowError("無法儲存檔案", err.Error())
		return false
	}
	if path == "" {
		return false
	}
	if filepath.Ext(path) == "" {
		path += ".xlsx"
	}
	return ds.saveTo(ctx, path)
}

func (ds *DocumentService) saveTo(ctx context.Context, path string) bool {
	// Saving only reads the workbook, but it marks the history saved and
	// may change the path, so take the write lock.
	ds.at(ctx).mu.Lock()
	err := ds.at(ctx).doc.SaveAs(path)
	ds.at(ctx).mu.Unlock()
	if err != nil {
		ds.dialogs.ShowError("無法儲存「"+filepath.Base(path)+"」", err.Error())
		return false
	}
	ds.at(ctx).PublishState()
	if ds.Opened != nil {
		ds.Opened(path)
	}
	return true
}

// RequestClose is called by the frontend after it has committed any
// in-progress edit.
func (ds *DocumentService) RequestClose(ctx context.Context) {
	if s := ds.at(ctx); s.CloseWindow != nil {
		s.CloseWindow()
	}
}

// ConfirmClose is called before the window closes. It returns true when
// it is OK to close.
func (ds *DocumentService) ConfirmClose(ctx context.Context) bool {
	return ds.confirmDiscard(ctx)
}

// confirmDiscard asks to save unsaved changes. It returns false if the
// user cancelled or the save failed.
func (ds *DocumentService) confirmDiscard(ctx context.Context) bool {
	if !ds.at(ctx).Dirty() {
		return true
	}
	var name string
	ds.at(ctx).read(func(d *document.Document) { name = d.Name() })
	switch ds.dialogs.AskSave(name) {
	case ChoiceSave:
		return ds.Save(ctx)
	case ChoiceDiscard:
		return true
	default:
		return false
	}
}

// ---- Sheets ----

// sheetChange runs a sheet operation as one undo step.
func (ds *DocumentService) sheetChange(ctx context.Context, build func(d *document.Document) (command.Command, error)) error {
	var err error
	ds.at(ctx).change(false, func(d *document.Document) (document.Change, bool) {
		var cmd command.Command
		if cmd, err = build(d); err != nil || cmd == nil {
			return document.Change{}, false
		}
		return d.Exec(cmd)
	})
	return err
}

// AddSheet inserts a new sheet (工作表N) at index and returns its index.
func (ds *DocumentService) AddSheet(ctx context.Context, index int) int {
	at := index
	ds.sheetChange(ctx, func(d *document.Document) (command.Command, error) {
		at = max(0, min(index, len(d.Workbook().Sheets)))
		return d.AddSheetCommand(at, d.NextSheetName())
	})
	return at
}

// DeleteSheet removes a sheet, asking first when it has data.
func (ds *DocumentService) DeleteSheet(ctx context.Context, index int) bool {
	var name string
	hasData := false
	ds.at(ctx).read(func(d *document.Document) {
		if sh := sheetAt(d, index); sh != nil {
			name = sh.Name
			rows, cols := sh.UsedRange()
			hasData = rows > 0 || cols > 0
		}
	})
	if hasData && !ds.dialogs.Confirm("要刪除工作表「"+name+"」嗎？", "工作表中的資料會一併刪除，參照它的公式會變成 #REF!。可以用「復原」還原。", "刪除") {
		return false
	}
	if err := ds.sheetChange(ctx, func(d *document.Document) (command.Command, error) { return d.DeleteSheetCommand(index) }); err != nil {
		ds.dialogs.ShowError("無法刪除工作表", err.Error())
		return false
	}
	return true
}

// RenameSheet renames a sheet; the error explains invalid names.
func (ds *DocumentService) RenameSheet(ctx context.Context, index int, name string) error {
	return ds.sheetChange(ctx, func(d *document.Document) (command.Command, error) { return d.RenameSheetCommand(index, name) })
}

func (ds *DocumentService) MoveSheet(ctx context.Context, from, to int) {
	ds.sheetChange(ctx, func(d *document.Document) (command.Command, error) { return d.MoveSheetCommand(from, to) })
}
