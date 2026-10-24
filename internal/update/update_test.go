package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.0.0", "1.0.0", false},
		{"v1.10.0", "1.9.9", true},
		{"1.0", "1.0.0", false},
		{"1.1.0", "1.1.0-beta.2", true},
		{"1.1.0-beta.2", "1.1.0", false},
		{"1.1.0-beta.3", "1.1.0-beta.2", true},
		{"0.9.0", "1.0.0", false},
	}
	for _, tc := range tests {
		if got := Newer(tc.a, tc.b); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v", tc.a, tc.b, got)
		}
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/app/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"tag_name":"v1.2.0","html_url":"https://example.com/r","body":"notes"}`))
	}))
	defer srv.Close()
	API = srv.URL
	defer func() { API = "https://api.github.com" }()

	rel, err := Latest(context.Background(), srv.Client(), "me/app")
	if err != nil || rel.Version != "1.2.0" || rel.URL != "https://example.com/r" {
		t.Fatalf("Latest = %+v, %v", rel, err)
	}
	if _, err := Latest(context.Background(), srv.Client(), "me/none"); !errors.Is(err, ErrNoRelease) {
		t.Fatalf("no release: err = %v", err)
	}
}
