// Package webui serves the built web app, which the release build embeds.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// dist holds the output of the web build. In a fresh checkout it contains
// only .gitkeep; scripts/build.sh fills it before compiling.
//
//go:embed all:dist
var dist embed.FS

const notice = "Keduly is running, but this binary was built without the web app.\n" +
	"Build it with scripts/build.sh, or use the API under /api/v1 and the keduly CLI.\n"

// Handler serves static files and falls back to index.html for app routes.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return handler(sub)
}

func handler(files fs.FS) http.Handler {
	server := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if isFile(files, name) && name != "index.html" {
			if strings.HasPrefix(name, "assets/") {
				// Vite puts a content hash in these names.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "public, max-age=3600")
			}
			server.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)
			return
		}
		index, err := fs.ReadFile(files, "index.html")
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(notice))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}

func isFile(files fs.FS, name string) bool {
	if name == "" || strings.HasPrefix(path.Base(name), ".") {
		return false
	}
	info, err := fs.Stat(files, name)
	return err == nil && !info.IsDir()
}
