package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ww-excel/internal/i18n"
	"ww-excel/internal/recovery"

	"ww-excel/internal/command"
	"ww-excel/internal/document"
)

type SaveChoice int

const (
	ChoiceSave SaveChoice = iota
	ChoiceDiscard
	ChoiceCancel
)

type CompatibilityChoice int

const (
	CompatibilityCancel CompatibilityChoice = iota
	CompatibilitySave
	CompatibilityCopy
)

// Dialogs abstracts native dialogs so the service can be tested.
type Dialogs interface {
	AskCompatibility(docName, message string) CompatibilityChoice
	// AskSave asks whether to save changes before discarding them.
	AskSave(docName string) SaveChoice
	// PickOpen returns "" when cancelled.
	PickOpen() (string, error)
	// PickSave returns "" when cancelled.
	PickSave(suggestedName string) (string, error)
	// PickImage asks for a picture file; PickPDF for where to export a
	// PDF. Both return "" when cancelled.
	PickImage() (string, error)
	PickPDF(suggestedName string) (string, error)
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
	// PrintPDF shows the system print dialog for a PDF file (set by main).
	PrintPDF func(path string)
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
		ds.dialogs.ShowError(fmt.Sprintf("無法開啟「%s」", filepath.Base(path)), err.Error())
		return nil, err
	}
	if ds.Opened != nil {
		ds.Opened(path)
	}
	if w := doc.TakeWarning(); w != "" {
		go ds.dialogs.ShowWarning(fmt.Sprintf("「%s」未完整載入", filepath.Base(path)), w)
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
	return ds.saveAs(ctx, name, "")
}

func (ds *DocumentService) saveAs(ctx context.Context, name, protectedPath string) bool {
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
	if protectedPath != "" && sameSavePath(path, protectedPath) {
		ds.dialogs.ShowError("無法另存副本", "副本必須使用不同的檔名或位置，才能保留原始檔案。")
		return false
	}
	return ds.saveTo(ctx, path)
}

func (ds *DocumentService) saveTo(ctx context.Context, path string) bool {
	s := ds.at(ctx)
	for {
		s.mu.RLock()
		doc, version := s.doc, s.doc.Version()
		risks := doc.SaveRisks(path)
		encoded, _ := json.Marshal(risks)
		absolute, _ := filepath.Abs(path)
		key := absolute + "\n" + string(encoded)
		confirmed := s.confirmedSaveRisks[key]
		sourcePath, name := doc.Path(), doc.Name()
		s.mu.RUnlock()

		if len(risks) > 0 && !confirmed {
			choice := ds.dialogs.AskCompatibility(filepath.Base(path), compatibilityMessage(risks))
			if choice != CompatibilitySave && choice != CompatibilityCopy {
				return false
			}
			s.mu.RLock()
			sameDoc, sameVersion := s.doc == doc, s.doc.Version() == version
			s.mu.RUnlock()
			if !sameDoc {
				return false
			}
			if !sameVersion {
				continue
			}
			switch choice {
			case CompatibilitySave:
			case CompatibilityCopy:
				// Prefer XLSX for a CSV copy so formulas and formatting can be kept.
				ext := filepath.Ext(path)
				if document.FormatForPath(path) == document.FormatCSV {
					ext = ".xlsx"
				}
				name = strings.TrimSuffix(name, filepath.Ext(name)) + i18n.T(" 副本") + ext
				if sourcePath == "" {
					sourcePath = path
				}
				return ds.saveAs(ctx, name, sourcePath)
			default:
				return false
			}
		}

		// Native dialogs must run without the document lock. Recheck the
		// document and edits before writing the state the user approved.
		s.mu.Lock()
		if s.doc != doc {
			s.mu.Unlock()
			return false
		}
		if s.doc.Version() != version {
			s.mu.Unlock()
			continue
		}
		err := doc.SaveAs(path)
		if err == nil && len(risks) > 0 {
			if s.confirmedSaveRisks == nil {
				s.confirmedSaveRisks = map[string]bool{}
			}
			s.confirmedSaveRisks[key] = true
		}
		s.mu.Unlock()
		if err != nil {
			ds.dialogs.ShowError(fmt.Sprintf("無法儲存「%s」", filepath.Base(path)), err.Error())
			return false
		}
		break
	}
	ds.at(ctx).PublishState()
	if ds.Opened != nil {
		ds.Opened(path)
	}
	return true
}

func compatibilityMessage(risks []document.SaveRisk) string {
	lines := []string{i18n.T("這次儲存有以下相容性影響："), ""}
	for i, risk := range risks {
		if i == 6 {
			lines = append(lines, fmt.Sprintf(i18n.T("另有 %d 項相同類型的影響。"), len(risks)-i))
			break
		}
		lines = append(lines, "• "+risk.Message)
	}
	lines = append(lines, "", i18n.T("另存副本可保留原始檔案，但副本仍可能有上述影響。"), i18n.T("本次開啟期間，相同儲存位置及相同影響不再重複提醒。"))
	return strings.Join(lines, "\n")
}

func sameSavePath(a, b string) bool {
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	if aa == bb {
		return true
	}
	ai, ae := os.Stat(a)
	bi, be := os.Stat(b)
	return ae == nil && be == nil && os.SameFile(ai, bi)
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
	if hasData && !ds.dialogs.Confirm(fmt.Sprintf("要刪除工作表「%s」嗎？", name), "工作表中的資料會一併刪除，參照它的公式會變成 #REF!。可以用「復原」還原。", "刪除") {
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

// showDocument puts doc in the calling window when it is a pristine new
// document, else in a new window.
func (ds *DocumentService) showDocument(ctx context.Context, doc *document.Document) {
	if s := ds.at(ctx); ds.OpenWindow == nil || (s != nil && s.Pristine()) {
		ds.at(ctx).replace(doc)
		return
	}
	ds.OpenWindow(doc)
}

// Recover offers to restore the snapshots a crashed session left in
// store, then opens or discards them. (A function, not a method: the
// service's methods are bound to the frontend.)
func Recover(ds *DocumentService, ctx context.Context, store *recovery.Store) {
	orphans := store.Orphans()
	if len(orphans) == 0 {
		return
	}
	names := make([]string, len(orphans))
	for i, e := range orphans {
		names[i] = "・" + e.Title
	}
	title := i18n.T("WW Excel 上次沒有正常結束")
	msg := fmt.Sprintf(i18n.T("有 %d 份文件的變更還沒儲存，要復原嗎？"), len(orphans)) + "\n\n" + strings.Join(names, "\n")
	restore := ds.dialogs.Confirm(title, msg, i18n.T("復原文件"))
	for _, e := range orphans {
		if restore {
			wb, err := store.Load(e)
			if err != nil {
				ds.dialogs.ShowError(fmt.Sprintf(i18n.T("無法復原「%s」"), e.Title), err.Error())
				continue
			}
			ds.showDocument(ctx, document.Recovered(wb, fmt.Sprintf(i18n.T("%s（已復原）"), e.Title)))
		}
		store.Remove(e.ID)
	}
}

// SetTabColor changes the worksheet tab color as one undoable edit.
func (ds *DocumentService) SetTabColor(ctx context.Context, sheet int, color string) error {
	return ds.sheetChange(ctx, func(d *document.Document) (command.Command, error) { return d.SetTabColorCommand(sheet, color) })
}
