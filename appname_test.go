package main

import (
	"testing"
	"ww-excel/internal/i18n"
)

func TestApplicationDisplayName(t *testing.T) {
	previous := i18n.Language()
	defer i18n.SetLanguage(previous)
	for _, tc := range []struct{ language, want string }{
		{i18n.Chinese, "旺-Sheet"}, {i18n.SimplifiedChinese, "旺-Sheet"}, {i18n.English, "Want-Sheet"},
	} {
		i18n.SetLanguage(tc.language)
		if got := applicationDisplayName(); got != tc.want {
			t.Errorf("%s: %q want %q", tc.language, got, tc.want)
		}
	}
}
