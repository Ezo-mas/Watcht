package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
)

func newTestRouter(t *testing.T) (http.Handler, *bytes.Buffer) {
	t.Helper()
	return newTestRouterWithHealth(t, nil)
}

func newTestRouterWithHealth(t *testing.T, health func(context.Context) error) (http.Handler, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "spa")
	})
	h, err := NewRouter(Deps{
		Logger: slog.New(slog.NewTextHandler(&logs, nil)),
		WebUI:  spa,
		Health: health,
		Auth:   auth.New(dbtest.New(t), auth.Options{Argon2: cheapArgon2}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, &logs
}

func TestHealthzReportsUnhealthyDependency(t *testing.T) {
	h, logs := newTestRouterWithHealth(t, func(context.Context) error {
		return errors.New("disk on fire at /data/watcht.db")
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "unhealthy" {
		t.Errorf("code = %q, want unhealthy", body.Error.Code)
	}
	if strings.Contains(rec.Body.String(), "/data") {
		t.Errorf("response leaks the internal error: %s", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "disk on fire") {
		t.Errorf("the cause should be logged: %s", logs.String())
	}
}

func TestHealthzHealthyDependency(t *testing.T) {
	h, _ := newTestRouterWithHealth(t, func(context.Context) error { return nil })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
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
