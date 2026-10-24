package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// MaxRecent is how many files 最近使用的檔案 remembers.
const MaxRecent = 10

// RecentFiles is the persisted most-recently-used file list, newest
// first.
type RecentFiles struct {
	mu    sync.Mutex
	path  string
	files []string
	// OnChange is called after the list changes (to rebuild the menu).
	OnChange func([]string)
}

// LoadRecent reads the list stored at path; a missing or broken file
// starts an empty list.
func LoadRecent(path string) *RecentFiles {
	r := &RecentFiles{path: path}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &r.files)
	}
	return r
}

// DefaultRecentPath is ~/Library/Application Support/ww-excel/recent.json.
func DefaultRecentPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "ww-excel", "recent.json")
}

func (r *RecentFiles) List() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.files...)
}

// Add moves path to the top of the list.
func (r *RecentFiles) Add(path string) {
	r.update(func(files []string) []string {
		out := []string{path}
		for _, f := range files {
			if f != path && len(out) < MaxRecent {
				out = append(out, f)
			}
		}
		return out
	})
}

// Remove drops a file, e.g. one that no longer exists.
func (r *RecentFiles) Remove(path string) {
	r.update(func(files []string) []string {
		var out []string
		for _, f := range files {
			if f != path {
				out = append(out, f)
			}
		}
		return out
	})
}

func (r *RecentFiles) Clear() { r.update(func([]string) []string { return nil }) }

func (r *RecentFiles) update(fn func([]string) []string) {
	r.mu.Lock()
	r.files = fn(r.files)
	files := append([]string(nil), r.files...)
	if r.path != "" {
		if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err == nil {
			data, _ := json.Marshal(files)
			_ = os.WriteFile(r.path, data, 0o644)
		}
	}
	cb := r.OnChange
	r.mu.Unlock()
	if cb != nil {
		cb(files)
	}
}
