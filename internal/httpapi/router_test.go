package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestRouter(t *testing.T) (http.Handler, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "spa")
	})
	return NewRouter(Deps{
		Logger: slog.New(slog.NewTextHandler(&logs, nil)),
		WebUI:  spa,
	}), &logs
}

func TestHealthz(t *testing.T) {
	h, _ := newTestRouter(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
}

func TestHealthzRejectsPost(t *testing.T) {
	h, _ := newTestRouter(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/healthz", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestUnknownAPIPathReturnsJSONError(t *testing.T) {
	for _, path := range []string{"/api/", "/api/v1/nope", "/api/v2/movies"} {
		t.Run(path, func(t *testing.T) {
			h, _ := newTestRouter(t)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", rec.Body.String(), err)
			}
			if body.Error.Code != "not_found" || body.Error.Message == "" {
				t.Errorf("error = %+v, want code not_found with a message", body.Error)
			}
		})
	}
}

func TestOtherPathsServeSPA(t *testing.T) {
	for _, path := range []string{"/", "/movies/1", "/apiary"} {
		t.Run(path, func(t *testing.T) {
			h, _ := newTestRouter(t)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Body.String() != "spa" {
				t.Errorf("body = %q, want the SPA handler", rec.Body.String())
			}
		})
	}
}

func TestRequestLogOmitsQueryAndHealthz(t *testing.T) {
	h, logs := newTestRouter(t)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/movies?token=secret123", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))

	out := logs.String()
	if strings.Contains(out, "secret123") || strings.Contains(out, "token=") {
		t.Errorf("log contains the query string: %s", out)
	}
	if !strings.Contains(out, "path=/movies") {
		t.Errorf("log is missing the request path: %s", out)
	}
	if strings.Contains(out, "/healthz") {
		t.Errorf("successful health checks should not be logged: %s", out)
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/healthz", nil))
	if !strings.Contains(logs.String(), "path=/healthz status=405") {
		t.Errorf("failed health check should be logged: %s", logs.String())
	}
}
