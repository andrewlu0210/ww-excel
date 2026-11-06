package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"ww-excel/internal/document"
)

const EventFileLoading = "doc:loading"

type FileLoading struct {
	ID     uint64 `json:"id"`
	Active bool   `json:"active"`
	Name   string `json:"name"`
}
type openJob struct {
	state     FileLoading
	cancel    context.CancelFunc
	committed bool
}

// Opening also covers imports started before the frontend subscribed.
func (ds *DocumentService) Opening(ctx context.Context) FileLoading {
	s := ds.at(ctx)
	ds.importMu.Lock()
	defer ds.importMu.Unlock()
	if j := ds.imports[s]; j != nil {
		return j.state
	}
	return FileLoading{ID: ds.importID}
}

// CancelOpen is independent of the editor queue and the document lock.
func (ds *DocumentService) CancelOpen(ctx context.Context) bool {
	s := ds.at(ctx)
	ds.importMu.Lock()
	defer ds.importMu.Unlock()
	if j := ds.imports[s]; j != nil && !j.committed {
		j.cancel()
		return true
	}
	return false
}
func (ds *DocumentService) openDocument(parent context.Context, path string, replace bool) error {
	s := ds.at(parent)
	if s == nil {
		return fmt.Errorf("沒有可開啟文件的視窗")
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	ds.importMu.Lock()
	if ds.imports == nil {
		ds.imports = map[*Session]*openJob{}
	}
	if old := ds.imports[s]; old != nil {
		old.cancel()
	}
	ds.importID++
	job := &openJob{state: FileLoading{ID: ds.importID, Active: true, Name: filepath.Base(path)}, cancel: cancel}
	ds.imports[s] = job
	ds.importMu.Unlock()
	s.emit(EventFileLoading, job.state)
	defer func() {
		cancel()
		ds.importMu.Lock()
		current := ds.imports[s] == job
		if current {
			delete(ds.imports, s)
		}
		ds.importMu.Unlock()
		if current {
			s.emit(EventFileLoading, FileLoading{ID: job.state.ID})
		}
	}()
	loader := ds.loadDocument
	if loader == nil {
		loader = document.LoadContext
	}
	doc, err := loader(ctx, path)
	if ctx.Err() != nil {
		if doc != nil {
			doc.Close()
			doc = nil
		}
		err = ctx.Err()
	}
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			message := err.Error()
			if errors.Is(err, context.DeadlineExceeded) {
				message = "匯入超過時間限制（2 分鐘），請縮小檔案後重試。"
			}
			ds.dialogs.ShowError(fmt.Sprintf("無法開啟「%s」", filepath.Base(path)), message)
		}
		return err
	}
	accept := func() bool {
		ds.importMu.Lock()
		defer ds.importMu.Unlock()
		if ds.imports[s] != job || ctx.Err() != nil {
			return false
		}
		job.committed = true
		return true
	}
	if replace {
		if !s.replaceIf(doc, accept) {
			return context.Canceled
		}
	} else {
		s.mu.RLock()
		closed := s.closed
		s.mu.RUnlock()
		if closed || !accept() {
			doc.Close()
			return context.Canceled
		}
		ds.OpenWindow(doc)
	}
	if ds.Opened != nil {
		ds.Opened(path)
	}
	if w := doc.TakeWarning(); w != "" {
		go ds.dialogs.ShowWarning(fmt.Sprintf("「%s」未完整載入", filepath.Base(path)), w)
	}
	return nil
}
