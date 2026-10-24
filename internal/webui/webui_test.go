package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                  {Data: []byte("<html>app</html>")},
		"favicon.png":                 {Data: []byte("png")},
		"_app/immutable/entry/app.js": {Data: []byte("console.log(1)")},
	}
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
		wantCache  string
	}{
		{name: "root serves index", method: http.MethodGet, path: "/", wantStatus: 200, wantBody: "<html>app</html>", wantCache: "no-cache"},
		{name: "client route falls back to index", method: http.MethodGet, path: "/movies/42", wantStatus: 200, wantBody: "<html>app</html>", wantCache: "no-cache"},
		{name: "index.html path serves index", method: http.MethodGet, path: "/index.html", wantStatus: 200, wantBody: "<html>app</html>", wantCache: "no-cache"},
		{name: "static file", method: http.MethodGet, path: "/favicon.png", wantStatus: 200, wantBody: "png"},
		{name: "immutable asset is cached", method: http.MethodGet, path: "/_app/immutable/entry/app.js", wantStatus: 200, wantBody: "console.log(1)", wantCache: "public, max-age=31536000, immutable"},
		{name: "missing build asset is a 404", method: http.MethodGet, path: "/_app/immutable/entry/old.js", wantStatus: 404},
		{name: "build directory is a 404", method: http.MethodGet, path: "/_app/immutable/", wantStatus: 404},
		{name: "other directory falls back to index", method: http.MethodGet, path: "/movies/", wantStatus: 200, wantBody: "<html>app</html>", wantCache: "no-cache"},
		{name: "path traversal stays inside", method: http.MethodGet, path: "/../../etc/passwd", wantStatus: 200, wantBody: "<html>app</html>", wantCache: "no-cache"},
		{name: "head has no body", method: http.MethodHead, path: "/", wantStatus: 200, wantBody: "", wantCache: "no-cache"},
		{name: "post not allowed", method: http.MethodPost, path: "/", wantStatus: 405},
	}
	h := Handler(testFS())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == 200 && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
			}
		})
	}
}

func TestHandlerWithoutBuild(t *testing.T) {
	h := Handler(fstest.MapFS{".gitkeep": {}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "make build") {
		t.Errorf("body = %q, want a hint to run make build", rec.Body.String())
	}
}

func TestDistIsReadable(t *testing.T) {
	// The embedded directory always exists (dist/.gitkeep is committed), so
	// the binary builds without a frontend build.
	if _, err := Dist().Open("."); err != nil {
		t.Fatalf("Dist().Open(\".\") error = %v", err)
	}
}

func TestWebmanifestMIME(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte("x")}, "app.webmanifest": {Data: []byte("{}")}}
	rec := httptest.NewRecorder()
	Handler(fsys).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app.webmanifest", nil))

	if ct := rec.Header().Get("Content-Type"); ct != "application/manifest+json" {
		t.Errorf("Content-Type = %q, want application/manifest+json", ct)
	}
}
