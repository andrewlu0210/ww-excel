package service

import (
	"ww-excel/internal/i18n"
	"ww-excel/internal/settings"
)

// EventSettingsChanged carries the new UISettings to every window.
const EventSettingsChanged = "settings:changed"

// UISettings is what the frontend needs: the stored preferences plus
// the language actually in use.
type UISettings struct {
	settings.Settings
	// Lang is "zh-TW" or "en" ("system" resolved).
	Lang string `json:"lang"`
}

// SettingsService exposes the preferences (⌘,) to the frontend.
type SettingsService struct {
	store *settings.Store
	// Preferred is the system's language list, for "system".
	Preferred []string
	// Changed is called after Set, e.g. to apply the language and
	// appearance natively and notify the windows. Set by main.
	Changed func(old, cur UISettings)
}

func NewSettingsService(store *settings.Store, preferred []string) *SettingsService {
	s := &SettingsService{store: store, Preferred: preferred}
	i18n.SetLanguage(s.Get().Lang)
	return s
}

func (s *SettingsService) ui(v settings.Settings) UISettings {
	return UISettings{Settings: v, Lang: settings.ResolveLanguage(v.Language, s.Preferred)}
}

func (s *SettingsService) Get() UISettings { return s.ui(s.store.Get()) }

// Set saves new preferences and applies them.
func (s *SettingsService) Set(v settings.Settings) (UISettings, error) {
	old := s.Get()
	saved, err := s.store.Set(v)
	cur := s.ui(saved)
	i18n.SetLanguage(cur.Lang)
	if s.Changed != nil {
		s.Changed(old, cur)
	}
	return cur, err
}

// Dictionary is the English UI table when English is in use, else
// empty (the source text is Chinese).
func (s *SettingsService) Dictionary() map[string]string {
	if s.Get().Lang != i18n.English {
		return map[string]string{}
	}
	return i18n.Dictionary()
}
