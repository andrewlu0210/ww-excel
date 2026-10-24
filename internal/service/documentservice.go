package service

import (
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
	s       *Session
	dialogs Dialogs

	// CloseWindow closes the document window; the window's closing hook
	// then asks about unsaved changes. Set by main.
	CloseWindow func()
}

func NewDocumentService(s *Session, dialogs Dialogs) *DocumentService {
	return &DocumentService{s: s, dialogs: dialogs}
}

func (ds *DocumentService) State() DocState {
	return ds.s.State()
}

// New replaces the document with an empty workbook.
func (ds *DocumentService) New() bool {
	if !ds.confirmDiscard() {
		return false
	}
	ds.s.replace(document.New())
	return true
}

// Open asks for a file and loads it.
func (ds *DocumentService) Open() bool {
	if !ds.confirmDiscard() {
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
	return ds.OpenPath(path) == nil
}

// OpenPath loads path without asking about unsaved changes. Errors are
// also shown to the user.
func (ds *DocumentService) OpenPath(path string) error {
	doc, err := document.Load(path)
	if err != nil {
		ds.dialogs.ShowError("無法開啟「"+filepath.Base(path)+"」", err.Error())
		return err
	}
	ds.s.replace(doc)
	if w := doc.TakeWarning(); w != "" {
		ds.dialogs.ShowWarning("「"+filepath.Base(path)+"」未完整載入", w)
	}
	return nil
}

// Save writes to the current path, asking for one if untitled.
func (ds *DocumentService) Save() bool {
	var path string
	ds.s.read(func(d *document.Document) { path = d.Path() })
	if path == "" {
		return ds.SaveAs()
	}
	return ds.saveTo(path)
}

func (ds *DocumentService) SaveAs() bool {
	var name string
	ds.s.read(func(d *document.Document) { name = d.Name() })
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
	return ds.saveTo(path)
}

func (ds *DocumentService) saveTo(path string) bool {
	// Saving only reads the workbook, but it marks the history saved and
	// may change the path, so take the write lock.
	ds.s.mu.Lock()
	err := ds.s.doc.SaveAs(path)
	ds.s.mu.Unlock()
	if err != nil {
		ds.dialogs.ShowError("無法儲存「"+filepath.Base(path)+"」", err.Error())
		return false
	}
	ds.s.PublishState()
	return true
}

// RequestClose is called by the frontend after it has committed any
// in-progress edit.
func (ds *DocumentService) RequestClose() {
	if ds.CloseWindow != nil {
		ds.CloseWindow()
	}
}

// ConfirmClose is called before the window closes. It returns true when
// it is OK to close.
func (ds *DocumentService) ConfirmClose() bool {
	return ds.confirmDiscard()
}

// confirmDiscard asks to save unsaved changes. It returns false if the
// user cancelled or the save failed.
func (ds *DocumentService) confirmDiscard() bool {
	if !ds.s.Dirty() {
		return true
	}
	var name string
	ds.s.read(func(d *document.Document) { name = d.Name() })
	switch ds.dialogs.AskSave(name) {
	case ChoiceSave:
		return ds.Save()
	case ChoiceDiscard:
		return true
	default:
		return false
	}
}

// ---- Sheets ----

// sheetChange runs a sheet operation as one undo step.
func (ds *DocumentService) sheetChange(build func(d *document.Document) (command.Command, error)) error {
	var err error
	ds.s.change(false, func(d *document.Document) (document.Change, bool) {
		var cmd command.Command
		if cmd, err = build(d); err != nil || cmd == nil {
			return document.Change{}, false
		}
		return d.Exec(cmd)
	})
	return err
}

// AddSheet inserts a new sheet (工作表N) at index and returns its index.
func (ds *DocumentService) AddSheet(index int) int {
	at := index
	ds.sheetChange(func(d *document.Document) (command.Command, error) {
		at = max(0, min(index, len(d.Workbook().Sheets)))
		return d.AddSheetCommand(at, d.NextSheetName())
	})
	return at
}

// DeleteSheet removes a sheet, asking first when it has data.
func (ds *DocumentService) DeleteSheet(index int) bool {
	var name string
	hasData := false
	ds.s.read(func(d *document.Document) {
		if sh := sheetAt(d, index); sh != nil {
			name = sh.Name
			rows, cols := sh.UsedRange()
			hasData = rows > 0 || cols > 0
		}
	})
	if hasData && !ds.dialogs.Confirm("要刪除工作表「"+name+"」嗎？", "工作表中的資料會一併刪除，參照它的公式會變成 #REF!。可以用「復原」還原。", "刪除") {
		return false
	}
	if err := ds.sheetChange(func(d *document.Document) (command.Command, error) { return d.DeleteSheetCommand(index) }); err != nil {
		ds.dialogs.ShowError("無法刪除工作表", err.Error())
		return false
	}
	return true
}

// RenameSheet renames a sheet; the error explains invalid names.
func (ds *DocumentService) RenameSheet(index int, name string) error {
	return ds.sheetChange(func(d *document.Document) (command.Command, error) { return d.RenameSheetCommand(index, name) })
}

func (ds *DocumentService) MoveSheet(from, to int) {
	ds.sheetChange(func(d *document.Document) (command.Command, error) { return d.MoveSheetCommand(from, to) })
}
