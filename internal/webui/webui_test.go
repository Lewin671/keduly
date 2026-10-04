package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestServesAppWithFallback(t *testing.T) {
	h := handler(fstest.MapFS{
		"index.html":          {Data: []byte("<html>app</html>")},
		"assets/app-abc12.js": {Data: []byte("console.log(1)")},
		"favicon.svg":         {Data: []byte("<svg/>")},
		".gitkeep":            {},
	})
	for _, path := range []string{"/", "/today", "/projects/abc/items", "/index.html", "/.gitkeep"} {
		rec := get(h, "GET", path)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "app") || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("GET %s: %d %q %q", path, rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
		}
	}
	rec := get(h, "GET", "/assets/app-abc12.js")
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") || rec.Body.String() != "console.log(1)" {
		t.Fatalf("asset: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if rec := get(h, "GET", "/favicon.svg"); rec.Code != 200 || strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("favicon: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	// A missing asset is an error, not the app shell.
	if rec := get(h, "GET", "/assets/gone.js"); rec.Code != 404 {
		t.Fatalf("missing asset: %d", rec.Code)
	}
	if rec := get(h, "POST", "/"); rec.Code != 405 {
		t.Fatalf("POST: %d", rec.Code)
	}
}

func TestNoticeWithoutBuild(t *testing.T) {
	rec := get(handler(fstest.MapFS{".gitkeep": {}}), "GET", "/")
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "without the web app") ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("notice: %d %q", rec.Code, rec.Body.String())
	}
	// The embedded directory is usable as it is in a fresh checkout.
	if rec := get(Handler(), "GET", "/"); rec.Code != 200 && rec.Code != 404 {
		t.Fatalf("embedded handler: %d", rec.Code)
	}
}
