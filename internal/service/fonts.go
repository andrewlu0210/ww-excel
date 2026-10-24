package service

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// FontSource returns an installed font as a font file (set by main on
// macOS). Pages cannot use macOS's downloadable fonts directly; the
// frontend loads them from /fonts/<family>.ttf instead.
var FontSource func(family string) ([]byte, error)

var fontCache sync.Map // family -> []byte (nil: not installed)

// FontHandler serves fonts at /fonts/<family>.ttf; other requests go on
// to next.
func FontHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/fonts/") {
			next.ServeHTTP(w, r)
			return
		}
		family, err := url.PathUnescape(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/fonts/"), ".ttf"))
		if err != nil || FontSource == nil {
			http.NotFound(w, r)
			return
		}
		cached, ok := fontCache.Load(family)
		if !ok {
			data, err := FontSource(family)
			if err != nil {
				data = nil
			}
			cached, _ = fontCache.LoadOrStore(family, data)
		}
		data, _ := cached.([]byte)
		if data == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "font/ttf")
		w.Header().Set("Cache-Control", "max-age=86400")
		_, _ = w.Write(data)
	})
}
