package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSetNormalize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	st := Load(path)
	if st.Get() != Defaults() {
		t.Fatalf("missing file = %+v", st.Get())
	}
	var notified Settings
	st.OnChange = func(s Settings) { notified = s }
	got, err := st.Set(Settings{Appearance: "dark", Language: "klingon", AutoRecoverMinutes: 999, EnterMove: "right"})
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{Appearance: "dark", Language: "system", AutoRecoverMinutes: 120, EnterMove: "right"}
	if got != want || notified != want {
		t.Fatalf("set = %+v, notified %+v", got, notified)
	}
	if again := Load(path).Get(); again != want {
		t.Fatalf("reloaded = %+v", again)
	}

	// Fields missing from an older file keep their defaults.
	os.WriteFile(path, []byte(`{"appearance":"light"}`), 0o644)
	if s := Load(path).Get(); s.Appearance != "light" || s.AutoRecoverMinutes != 2 || !s.CheckUpdates {
		t.Fatalf("partial file = %+v", s)
	}
}

func TestResolveLanguage(t *testing.T) {
	tests := []struct {
		setting   string
		preferred []string
		want      string
	}{
		{"en", []string{"zh-Hant-TW"}, "en"},
		{"zh-TW", []string{"en-US"}, "zh-TW"},
		{"system", []string{"zh-Hant-TW", "en-US"}, "zh-TW"},
		{"system", []string{"zh-Hans-CN"}, "zh-TW"},
		{"system", []string{"en-GB", "zh-Hant"}, "en"},
		{"system", []string{"ja-JP"}, "en"},
		{"system", []string{"ja-JP", "zh-TW"}, "zh-TW"},
		{"system", nil, "zh-TW"},
	}
	for _, tc := range tests {
		if got := ResolveLanguage(tc.setting, tc.preferred); got != tc.want {
			t.Errorf("ResolveLanguage(%q, %v) = %q, want %q", tc.setting, tc.preferred, got, tc.want)
		}
	}
}
