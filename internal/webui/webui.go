// Package webui serves the SvelteKit single-page app that is embedded into
// the binary.
//
// `make build` copies the frontend build (web/build) into dist/ before
// compiling Go. Without that step dist/ holds only .gitkeep, the binary
// still builds, and every page returns 503 explaining how to build the
// frontend.
package webui

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// Dist returns the embedded frontend build.
func Dist() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		// fs.Sub only fails for an invalid path, and "dist" is a constant.
		panic(err)
	}
	return sub
}

const indexFile = "index.html"

// Handler serves files from fsys and falls back to index.html for any path
// that isn't a file, so the SPA's client-side router handles it. Callers
// route /api/ and /healthz elsewhere before reaching this handler.
func Handler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != indexFile && isFile(fsys, name) {
			if strings.HasPrefix(name, "_app/immutable/") {
				// Vite content-hashes these file names, so they never change.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}
		serveIndex(w, r, fsys)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	index, err := fs.ReadFile(fsys, indexFile)
	if errors.Is(err, fs.ErrNotExist) {
		http.Error(w, "watcht: the frontend is not built into this binary; run `make build`", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	// index.html references the current hashed assets, so browsers must
	// revalidate it after an upgrade.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(index)
	}
}

func isFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
