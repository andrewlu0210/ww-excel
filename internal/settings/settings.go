// Package settings holds the user preferences, persisted as JSON in
// ~/Library/Application Support/ww-excel/settings.json.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type Settings struct {
	// Appearance is "system", "light" or "dark".
	Appearance string `json:"appearance"`
	// Language is "system", "zh-TW", "zh-CN" or "en".
	Language string `json:"language"`
	// AutoRecoverMinutes is the autosave interval; 0 turns it off.
	AutoRecoverMinutes int `json:"autoRecoverMinutes"`
	// EnterMove is where Enter moves the selection: "down", "right",
	// "up", "left" or "none".
	EnterMove string `json:"enterMove"`
	// CheckUpdates looks for a newer release at launch.
	CheckUpdates bool `json:"checkUpdates"`
}

func Defaults() Settings {
	return Settings{Appearance: "system", Language: "system", AutoRecoverMinutes: 2, EnterMove: "down", CheckUpdates: true}
}

// Normalize replaces unknown or out-of-range values with defaults.
func (s Settings) Normalize() Settings {
	d := Defaults()
	if !slices.Contains([]string{"system", "light", "dark"}, s.Appearance) {
		s.Appearance = d.Appearance
	}
	if !slices.Contains([]string{"system", "zh-TW", "zh-CN", "en"}, s.Language) {
		s.Language = d.Language
	}
	if !slices.Contains([]string{"down", "right", "up", "left", "none"}, s.EnterMove) {
		s.EnterMove = d.EnterMove
	}
	s.AutoRecoverMinutes = min(max(s.AutoRecoverMinutes, 0), 120)
	return s
}

// Store is the persisted settings; safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	path string
	cur  Settings
	// OnChange is called after Set with the new settings.
	OnChange func(Settings)
}

// Load reads the settings at path; missing fields keep their defaults.
func Load(path string) *Store {
	s := Defaults()
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return &Store{path: path, cur: s.Normalize()}
}

// DefaultPath is ~/Library/Application Support/ww-excel/settings.json.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "ww-excel", "settings.json")
}

func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.cur
}

// Set stores and persists new settings, returning them normalized.
func (st *Store) Set(s Settings) (Settings, error) {
	s = s.Normalize()
	st.mu.Lock()
	st.cur = s
	path := st.path
	st.mu.Unlock()
	var err error
	if path != "" {
		err = save(path, s)
	}
	if st.OnChange != nil {
		st.OnChange(s)
	}
	return s, err
}

func save(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ResolveLanguage turns "system" into a concrete UI language given the
// system's preferred languages (e.g. ["zh-Hant-TW", "en-US"]): Chinese
// resolves its script/region; everything else defaults to English.
func ResolveLanguage(setting string, preferred []string) string {
	if setting == "zh-TW" || setting == "zh-CN" || setting == "en" {
		return setting
	}
	for _, p := range preferred {
		p = strings.ReplaceAll(strings.ToLower(p), "_", "-")
		if strings.HasPrefix(p, "zh") {
			if strings.Contains(p, "-hans") {
				return "zh-CN"
			}
			if strings.Contains(p, "-hant") {
				return "zh-TW"
			}
			for _, part := range strings.Split(p, "-")[1:] {
				if part == "cn" || part == "sg" {
					return "zh-CN"
				}
			}
			return "zh-TW"
		}
		if strings.HasPrefix(p, "en") {
			return "en"
		}
	}
	if len(preferred) == 0 {
		return "zh-TW"
	}
	return "en"
}
