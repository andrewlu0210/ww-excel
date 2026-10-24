// Package recovery keeps autosave snapshots of unsaved documents so they
// can be restored after a crash.
//
// Each document has a pair of files in the store directory: <id>.wwx
// with the workbook and <id>.json with its metadata, including the PID
// of the process that wrote it. Snapshots are removed when the document
// is saved or closed; any left behind by a process that is no longer
// running belong to a session that ended abnormally.
package recovery

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"ww-excel/internal/io/wwx"
	"ww-excel/internal/model"
)

type Entry struct {
	ID string `json:"id"`
	// Title is the document's name when the snapshot was taken.
	Title string `json:"title"`
	// Path is the file the document was opened from, if any.
	Path  string    `json:"path,omitempty"`
	Saved time.Time `json:"saved"`
	PID   int       `json:"pid"`
}

type Store struct {
	Dir string
	// PID is written into snapshots; snapshots with our own PID are
	// never orphans.
	PID int
	// Alive reports whether a process is running; injectable for tests.
	Alive func(pid int) bool
}

func New(dir string) *Store {
	return &Store{Dir: dir, PID: os.Getpid(), Alive: processAlive}
}

// DefaultDir is ~/Library/Application Support/ww-excel/Recovery.
func DefaultDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "ww-excel", "Recovery")
}

// NewID returns a fresh snapshot ID.
func NewID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Encode serializes a workbook for Save. It is separate so callers can
// encode while holding the document lock and write the file after.
func Encode(wb *model.Workbook) ([]byte, error) {
	var buf bytes.Buffer
	if err := wwx.Write(&buf, wb); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Save writes a snapshot. The workbook file is written before the
// metadata, so a listed entry always has its data.
func (s *Store) Save(e Entry, data []byte) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	e.PID = s.PID
	if e.Saved.IsZero() {
		e.Saved = time.Now()
	}
	meta, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := writeAtomic(s.file(e.ID, ".wwx"), data); err != nil {
		return err
	}
	return writeAtomic(s.file(e.ID, ".json"), meta)
}

// Remove deletes a snapshot; a missing one is not an error.
func (s *Store) Remove(id string) {
	os.Remove(s.file(id, ".json"))
	os.Remove(s.file(id, ".wwx"))
}

// Orphans lists snapshots left by processes that are no longer running,
// newest first.
func (s *Store) Orphans() []Entry {
	matches, _ := filepath.Glob(filepath.Join(s.Dir, "*.json"))
	var out []Entry
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		var e Entry
		if json.Unmarshal(data, &e) != nil || e.ID == "" || e.ID != strings.TrimSuffix(filepath.Base(m), ".json") {
			continue
		}
		if e.PID == s.PID || s.Alive(e.PID) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Saved.After(out[j].Saved) })
	return out
}

// Load reads a snapshot's workbook.
func (s *Store) Load(e Entry) (*model.Workbook, error) {
	f, err := os.Open(s.file(e.ID, ".wwx"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return wwx.Read(f)
}

func (s *Store) file(id, ext string) string {
	return filepath.Join(s.Dir, filepath.Base(id)+ext)
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	// EPERM: the process exists but belongs to someone else.
	return err == nil || errors.Is(err, syscall.EPERM)
}
