package auth_test

// These HTTP tests are here to access auth's test-only hash-slot helper;
// Go does not expose another package's _test.go helpers to httpapi tests.

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/httpapi"
)

func busyServer(t *testing.T, limiter *auth.Limiter) (http.Handler, *auth.Service, *http.Cookie, *bytes.Buffer) {
	t.Helper()
	service := auth.New(dbtest.New(t), auth.Options{Argon2: auth.Argon2Params{Memory: 64, Time: 1, Threads: 1}})
	var logs bytes.Buffer
	handler, err := httpapi.NewRouter(httpapi.Deps{
		Auth: service, WebUI: http.NotFoundHandler(), LoginLimiter: limiter,
		LoginWait: 5 * time.Millisecond,
		Logger:    slog.New(slog.NewTextHandler(&logs, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := busyRequest(handler, "POST", "/api/v1/auth/setup", `{"username":"admin","password":"long enough"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup: status %d, body %s", rec.Code, rec.Body)
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "watcht_session" {
			return handler, service, cookie, &logs
		}
	}
	t.Fatal("setup returned no session cookie")
	return nil, nil, nil, nil
}

func busyRequest(handler http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertBusy(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid error response: %d %s: %v", rec.Code, rec.Body, err)
	}
	if rec.Code != http.StatusServiceUnavailable || body.Error.Code != "busy" {
		t.Fatalf("response: %d %s, want 503 busy", rec.Code, rec.Body)
	}
}

// A login that runs out of time waiting for a hash slot is the server's
// problem: 503, Retry-After, and the attempt isn't counted.
func TestLoginBusy(t *testing.T) {
	t.Parallel()
	handler, service, _, _ := busyServer(t, auth.NewLimiter(1, time.Hour, nil))
	release := auth.HoldHashSlotsForTest(service)
	defer release()
	for range 3 {
		rec := busyRequest(handler, "POST", "/api/v1/auth/login", `{"username":"admin","password":"long enough"}`, nil)
		assertBusy(t, rec)
		if rec.Header().Get("Retry-After") != "5" {
			t.Errorf("Retry-After = %q, want 5", rec.Header().Get("Retry-After"))
		}
	}
}

// Account endpoints that hash a password also return 503 while all slots
// stay busy until the deadline.
func TestAccountEndpointsBusy(t *testing.T) {
	t.Parallel()
	handler, service, admin, logs := busyServer(t, nil)
	release := auth.HoldHashSlotsForTest(service)
	defer release()
	for _, req := range []struct{ method, path, body string }{
		{"POST", "/api/v1/users", `{"username":"bob","password":"long enough"}`},
		{"PUT", "/api/v1/me/password", `{"current_password":"long enough","new_password":"new password"}`},
	} {
		rec := busyRequest(handler, req.method, req.path, req.body, admin)
		assertBusy(t, rec)
	}
	if strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("a busy server logged errors:\n%s", logs)
	}
}
