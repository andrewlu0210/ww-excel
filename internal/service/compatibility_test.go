package service

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ww-excel/internal/document"
)

type compatibilityDialogs struct {
	fakeDialogs
	responses []CompatibilityChoice
	messages  []string
	suggested string
	onAsk     func()
}

func (f *compatibilityDialogs) AskCompatibility(name, message string) CompatibilityChoice {
	f.messages = append(f.messages, message)
	if f.onAsk != nil {
		f.onAsk()
	}
	if len(f.responses) == 0 {
		return CompatibilityCancel
	}
	choice := f.responses[0]
	f.responses = f.responses[1:]
	return choice
}

func (f *compatibilityDialogs) PickSave(name string) (string, error) {
	f.suggested = name
	return f.savePath, nil
}

func TestCompatibilityCancelAndRemember(t *testing.T) {
	sess, _ := newTestSession(false)
	sheets := NewSheetService(sess)
	sheets.SetCell(bg, 0, 0, 0, "=1+2")
	dlg := &compatibilityDialogs{responses: []CompatibilityChoice{CompatibilityCancel, CompatibilitySave}}
	docs := NewDocumentService(sess, dlg)
	path := filepath.Join(t.TempDir(), "out.csv")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	if docs.saveTo(bg, path) {
		t.Fatal("cancel must abort save")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" || !sess.Dirty() || sess.State().Path != "" {
		t.Fatal("cancel changed disk or document")
	}
	if !docs.saveTo(bg, path) || sess.Dirty() {
		t.Fatalf("continue failed: %v", dlg.errors)
	}
	data, _ = os.ReadFile(path)
	if !bytes.Contains(data, []byte("3\r\n")) {
		t.Fatalf("unexpected CSV: %q", data)
	}
	sheets.SetCell(bg, 0, 1, 0, "text")
	if !docs.Save(bg) || len(dlg.messages) != 2 {
		t.Fatal("same confirmed risk should not prompt again")
	}
	sheets.SetCell(bg, 0, 2, 0, "12%")
	if docs.Save(bg) || len(dlg.messages) != 3 || !sess.Dirty() {
		t.Fatal("new formatting risk must prompt")
	}
	if !strings.Contains(dlg.messages[0], "公式") {
		t.Fatal("missing concrete risk")
	}
}

func TestCompatibilityCopyAndClose(t *testing.T) {
	dir := t.TempDir()
	sess, _ := newTestSession(false)
	sheets := NewSheetService(sess)
	dlg := &compatibilityDialogs{}
	docs := NewDocumentService(sess, dlg)
	original := filepath.Join(dir, "source.xlsx")
	sheets.SetCell(bg, 0, 0, 0, "=1+2")
	if !docs.saveTo(bg, original) {
		t.Fatal(dlg.errors)
	}
	before, _ := os.ReadFile(original)
	sheets.SetCell(bg, 0, 1, 0, "new")
	dlg.responses = []CompatibilityChoice{CompatibilityCopy}
	dlg.savePath = filepath.Join(dir, "copy.xlsx")
	if !docs.saveTo(bg, filepath.Join(dir, "export.csv")) {
		t.Fatal(dlg.errors)
	}
	after, _ := os.ReadFile(original)
	if !bytes.Equal(before, after) || sess.State().Path != dlg.savePath || !strings.HasSuffix(dlg.suggested, ".xlsx") {
		t.Fatal("copy did not protect source")
	}
	if _, err := os.Stat(filepath.Join(dir, "export.csv")); !os.IsNotExist(err) {
		t.Fatal("copy must not write CSV")
	}

	// A rejected compatibility prompt must also stop Save-and-Close.
	dlg.responses = []CompatibilityChoice{CompatibilitySave}
	if !docs.saveTo(bg, filepath.Join(dir, "export.csv")) {
		t.Fatal(dlg.errors)
	}
	sheets.SetCell(bg, 0, 2, 0, "20%")
	dlg.choice = ChoiceSave
	if docs.ConfirmClose(bg) || !sess.Dirty() {
		t.Fatal("cancelled compatibility must stop close")
	}
}

func TestCompatibilityCopyCannotOverwriteSource(t *testing.T) {
	sess, _ := newTestSession(false)
	dlg := &compatibilityDialogs{responses: []CompatibilityChoice{CompatibilityCopy}}
	docs := NewDocumentService(sess, dlg)
	NewSheetService(sess).SetCell(bg, 0, 0, 0, "=1+2")
	path := filepath.Join(t.TempDir(), "source.xlsx")
	if !docs.saveTo(bg, path) {
		t.Fatal(dlg.errors)
	}
	dlg.savePath = path
	if docs.saveTo(bg, filepath.Join(t.TempDir(), "export.csv")) || len(dlg.errors) == 0 {
		t.Fatal("copy cannot overwrite original")
	}
}

func TestCompatibilityFailedSaveDoesNotRemember(t *testing.T) {
	sess, _ := newTestSession(false)
	NewSheetService(sess).SetCell(bg, 0, 0, 0, "=1+2")
	dlg := &compatibilityDialogs{responses: []CompatibilityChoice{CompatibilitySave}}
	docs := NewDocumentService(sess, dlg)
	path := filepath.Join(t.TempDir(), "missing", "out.csv")
	if docs.saveTo(bg, path) || !sess.Dirty() {
		t.Fatal("save should fail")
	}
	if docs.saveTo(bg, path) || len(dlg.messages) != 2 {
		t.Fatal("failed save must not acknowledge risk")
	}
}

func TestCompatibilityRechecksEditsAndDocument(t *testing.T) {
	sess, _ := newTestSession(false)
	sheets := NewSheetService(sess)
	sheets.SetCell(bg, 0, 0, 0, "=1+2")
	dlg := &compatibilityDialogs{responses: []CompatibilityChoice{CompatibilitySave}}
	docs := NewDocumentService(sess, dlg)
	dlg.onAsk = func() {
		dlg.onAsk = nil
		sheets.SetCell(bg, 0, 1, 0, "20%")
	}
	path := filepath.Join(t.TempDir(), "out.csv")
	if docs.saveTo(bg, path) || len(dlg.messages) != 2 {
		t.Fatal("changed edits must be checked again")
	}
	dlg.onAsk = func() { sess.replace(document.New()) }
	if docs.saveTo(bg, path) {
		t.Fatal("replaced document must not be saved under old path")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cancelled save wrote a file")
	}
}
