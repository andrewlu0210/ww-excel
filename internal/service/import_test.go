package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/document"
	"ww-excel/internal/importguard"
	"ww-excel/internal/model"
)

func awaitOpen(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("import did not stop")
		return nil
	}
}
func TestImportCancelPreservesStateAndNextOpen(t *testing.T) {
	s, _ := newTestSession(false)
	defer s.Close()
	sheets := NewSheetService(s)
	sheets.SetCell(bg, 0, 0, 0, "unsaved")
	before := s.State()
	dlg := &fakeDialogs{}
	ds := NewDocumentService(s, dlg)
	recent := 0
	ds.Opened = func(string) { recent++ }
	entered := make(chan struct{})
	ds.loadDocument = func(ctx context.Context, path string) (*document.Document, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	done := make(chan error, 1)
	go func() { done <- ds.OpenPath(bg, "slow.xlsx") }()
	<-entered
	if loading := ds.Opening(bg); !loading.Active || loading.Name != "slow.xlsx" {
		t.Fatalf("missing loading state: %+v", loading)
	}
	if !ds.CancelOpen(bg) {
		t.Fatal("cancel rejected")
	}
	if err := awaitOpen(t, done); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if ds.Opening(bg).Active || ds.CancelOpen(bg) {
		t.Fatal("finished import remains active")
	}
	if !reflect.DeepEqual(before, s.State()) || recent != 0 || len(dlg.errors) != 0 {
		t.Fatalf("cancel changed document/recent/errors: %v", dlg.errors)
	}
	sheets.SetCell(bg, 0, 1, 0, "still editable")
	sheets.Undo(bg)
	if sheets.EditText(bg, 0, 0, 0) != "unsaved" || sheets.EditText(bg, 0, 1, 0) != "" {
		t.Fatal("cancel broke undo")
	}
	ds.loadDocument = func(context.Context, string) (*document.Document, error) { return document.New(), nil }
	if err := ds.OpenPath(bg, "good.xlsx"); err != nil || recent != 1 {
		t.Fatalf("next open failed: %v", err)
	}
}
func TestImportSupersededLoaderCannotCommit(t *testing.T) {
	s, _ := newTestSession(false)
	defer s.Close()
	dlg := &fakeDialogs{}
	ds := NewDocumentService(s, dlg)
	entered, release := make(chan struct{}), make(chan struct{})
	ds.loadDocument = func(ctx context.Context, path string) (*document.Document, error) {
		if path == "old.xlsx" {
			close(entered)
			<-release
		}
		return document.FromWorkbook(model.NewWorkbook(), path), nil
	}
	var mu sync.Mutex
	var recent []string
	ds.Opened = func(path string) { mu.Lock(); recent = append(recent, path); mu.Unlock() }
	oldDone := make(chan error, 1)
	go func() { oldDone <- ds.OpenPath(bg, "old.xlsx") }()
	<-entered
	oldID := ds.Opening(bg).ID
	if err := ds.OpenPath(bg, "new.xlsx"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := awaitOpen(t, oldDone); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if s.doc.Path() != "new.xlsx" || !reflect.DeepEqual(recent, []string{"new.xlsx"}) || ds.Opening(bg).ID <= oldID || ds.Opening(bg).Active {
		t.Fatalf("stale import committed: %v", recent)
	}
}
func TestImportClosedSessionCannotCommit(t *testing.T) {
	s, _ := newTestSession(false)
	ds := NewDocumentService(s, &fakeDialogs{})
	entered, release := make(chan struct{}), make(chan struct{})
	ds.loadDocument = func(context.Context, string) (*document.Document, error) {
		close(entered)
		<-release
		return document.New(), nil
	}
	done := make(chan error, 1)
	go func() { done <- ds.OpenPath(bg, "late.xlsx") }()
	<-entered
	s.Close()
	close(release)
	if err := awaitOpen(t, done); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if s.doc.Path() != "" {
		t.Fatal("closed document replaced")
	}
}
func TestImportLimitPreservesOtherDocumentsAndCleansSnapshots(t *testing.T) {
	s, _ := newTestSession(false)
	defer s.Close()
	other, _ := newTestSession(false)
	defer other.Close()
	NewSheetService(s).SetCell(bg, 0, 0, 0, "unsaved")
	NewSheetService(other).SetCell(bg, 0, 0, 0, "other")
	before, otherBefore := s.State(), other.State()
	dlg := &fakeDialogs{}
	ds := NewDocumentService(s, dlg)
	file := filepath.Join(t.TempDir(), "small.xlsx")
	f := excelize.NewFile()
	f.SetCellStr("Sheet1", "A1", "hello")
	if err := f.SaveAs(file); err != nil {
		t.Fatal(err)
	}
	f.Close()
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	l := importguard.DefaultLimits()
	l.Cells = 0
	err := ds.OpenPath(importguard.WithLimits(bg, l), file)
	var limit *importguard.LimitError
	if !errors.As(err, &limit) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, s.State()) || !reflect.DeepEqual(otherBefore, other.State()) || len(dlg.errors) != 1 {
		t.Fatal("failed import changed documents")
	}
	entries, _ := os.ReadDir(temp)
	if len(entries) != 0 {
		t.Fatalf("leaked temporary files: %v", entries)
	}
	if err = ds.OpenPath(bg, file); err != nil {
		t.Fatal(err)
	}
	entries, _ = os.ReadDir(temp)
	if len(entries) != 0 {
		t.Fatalf("successful import leaked snapshots: %v", entries)
	}
}

func TestCancelledParserErrorIsSilent(t *testing.T) {
	s, _ := newTestSession(false)
	defer s.Close()
	dlg := &fakeDialogs{}
	ds := NewDocumentService(s, dlg)
	entered := make(chan struct{})
	ds.loadDocument = func(ctx context.Context, path string) (*document.Document, error) {
		close(entered)
		<-ctx.Done()
		return nil, errors.New("late parser error")
	}
	done := make(chan error, 1)
	go func() { done <- ds.OpenPath(bg, "cancelled.xlsx") }()
	<-entered
	ds.CancelOpen(bg)
	if err := awaitOpen(t, done); !errors.Is(err, context.Canceled) || len(dlg.errors) != 0 {
		t.Fatalf("cancel showed stale parser failure: %v, %v", err, dlg.errors)
	}
}
