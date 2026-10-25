package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
)

var cheapArgon2 = auth.Argon2Params{Memory: 64, Time: 1, Threads: 1}

type testClock struct{ t time.Time }

func (c *testClock) Now() time.Time { return c.t }

// authServer is a router with a real database and a controllable clock.
type authServer struct {
	t     *testing.T
	h     http.Handler
	clock *testClock
	logs  *bytes.Buffer
}

type authServerOptions struct {
	baseOrigin string
	proxies    []netip.Prefix
	limiter    *auth.Limiter
}

func newAuthServer(t *testing.T, opts authServerOptions) *authServer {
	t.Helper()
	clock := &testClock{t: time.Now().UTC().Truncate(time.Second)}
	var logs bytes.Buffer
	h, err := NewRouter(Deps{
		Logger:         slog.New(slog.NewTextHandler(&logs, nil)),
		WebUI:          http.NotFoundHandler(),
		Auth:           auth.New(dbtest.New(t), auth.Options{Argon2: cheapArgon2, Now: clock.Now}),
		LoginLimiter:   opts.limiter,
		BaseOrigin:     opts.baseOrigin,
		TrustedProxies: opts.proxies,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &authServer{t: t, h: h, clock: clock, logs: &logs}
}

// request is one HTTP request. Requests come from a same-origin browser
// page unless crossSite is set.
type request struct {
	method, path, body string
	cookie             *http.Cookie
	header             map[string]string
	remoteAddr         string
	crossSite          bool
}

func (s *authServer) do(req request) *httptest.ResponseRecorder {
	s.t.Helper()
	var body io.Reader
	if req.body != "" {
		body = strings.NewReader(req.body)
	}
	r := httptest.NewRequest(req.method, req.path, body)
	if req.body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if req.crossSite {
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		r.Header.Set("Origin", "https://evil.example")
	} else {
		r.Header.Set("Sec-Fetch-Site", "same-origin")
	}
	if req.cookie != nil {
		r.AddCookie(req.cookie)
	}
	for k, v := range req.header {
		r.Header.Set(k, v)
	}
	if req.remoteAddr != "" {
		r.RemoteAddr = req.remoteAddr
	}
	rec := httptest.NewRecorder()
	s.h.ServeHTTP(rec, r)
	return rec
}

func (s *authServer) setup(username, password string) *http.Cookie {
	s.t.Helper()
	rec := s.do(request{method: "POST", path: "/api/v1/auth/setup", body: creds(username, password)})
	if rec.Code != http.StatusCreated {
		s.t.Fatalf("setup: status %d, body %s", rec.Code, rec.Body)
	}
	return sessionCookieFrom(s.t, rec)
}

func creds(username, password string) string {
	b, _ := json.Marshal(map[string]string{"username": username, "password": password})
	return string(b)
}

func sessionCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatalf("no %s cookie in the response (Set-Cookie: %q)", sessionCookie, rec.Header().Values("Set-Cookie"))
	return nil
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorDetail {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code == "" {
		t.Fatalf("response isn't the standard error shape: %d %s", rec.Code, rec.Body)
	}
	return body.Error
}

type sessionResponse struct {
	SetupRequired bool      `json:"setup_required"`
	User          *userJSON `json:"user"`
}

func (s *authServer) session(cookie *http.Cookie) (sessionResponse, *httptest.ResponseRecorder) {
	s.t.Helper()
	rec := s.do(request{method: "GET", path: "/api/v1/auth/session", cookie: cookie})
	if rec.Code != http.StatusOK {
		s.t.Fatalf("GET session: status %d, body %s", rec.Code, rec.Body)
	}
	var resp sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		s.t.Fatal(err)
	}
	return resp, rec
}

func TestSetupFlow(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})

	resp, _ := s.session(nil)
	if !resp.SetupRequired || resp.User != nil {
		t.Fatalf("fresh instance: %+v, want setup_required and no user", resp)
	}

	rec := s.do(request{method: "POST", path: "/api/v1/auth/setup", body: creds("admin", "long enough")})
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup: status %d, body %s", rec.Code, rec.Body)
	}
	var created struct{ User userJSON }
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.User.Username != "admin" || !created.User.IsAdmin || created.User.ID == 0 {
		t.Errorf("setup user = %+v", created.User)
	}
	if strings.Contains(rec.Body.String(), "long enough") {
		t.Error("the response contains the password")
	}

	c := sessionCookieFrom(t, rec)
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Secure {
		t.Errorf("cookie = %+v, want HttpOnly, SameSite=Lax, Path=/, not Secure over plain HTTP", c)
	}
	if want := s.clock.Now().Add(auth.DefaultSessionTTL); !c.Expires.Equal(want) {
		t.Errorf("cookie expires %v, want %v", c.Expires, want)
	}
	if strings.Contains(rec.Body.String(), c.Value) {
		t.Error("the response body contains the session token")
	}

	resp, _ = s.session(c)
	if resp.SetupRequired || resp.User == nil || resp.User.Username != "admin" {
		t.Errorf("after setup: %+v, want signed in as admin", resp)
	}

	rec = s.do(request{method: "POST", path: "/api/v1/auth/setup", body: creds("second", "long enough")})
	if rec.Code != http.StatusConflict || decodeError(t, rec).Code != "setup_done" {
		t.Errorf("second setup: %d %s, want 409 setup_done", rec.Code, rec.Body)
	}
}

func TestSetupRejectsInvalidInput(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	for name, body := range map[string]string{
		"bad username":   creds("has space", "long enough"),
		"short password": creds("admin", "short"),
	} {
		rec := s.do(request{method: "POST", path: "/api/v1/auth/setup", body: body})
		if rec.Code != http.StatusBadRequest || decodeError(t, rec).Code != "invalid_input" {
			t.Errorf("%s: %d %s, want 400 invalid_input", name, rec.Code, rec.Body)
		}
	}
	if resp, _ := s.session(nil); !resp.SetupRequired {
		t.Error("rejected setup created an account")
	}
}

func TestLoginAndLogout(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	setupCookie := s.setup("Alice", "alice password")

	rec := s.do(request{method: "POST", path: "/api/v1/auth/login", body: creds("alice", "alice password")})
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	c := sessionCookieFrom(t, rec)
	if c.Value == setupCookie.Value {
		t.Error("login reused the existing session token")
	}
	if resp, _ := s.session(c); resp.User == nil || resp.User.Username != "Alice" {
		t.Errorf("after login: %+v", resp)
	}

	rec = s.do(request{method: "POST", path: "/api/v1/auth/logout", cookie: c})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	if cleared := sessionCookieFrom(t, rec); cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Errorf("logout cookie = %+v, want it deleted", cleared)
	}
	// The old cookie no longer works, even if the browser kept it.
	resp, rec := s.session(c)
	if resp.User != nil {
		t.Errorf("after logout: still signed in as %+v", resp.User)
	}
	if cleared := sessionCookieFrom(t, rec); cleared.MaxAge >= 0 {
		t.Error("a stale cookie wasn't cleared")
	}
	// The other session is untouched.
	if resp, _ := s.session(setupCookie); resp.User == nil {
		t.Error("logout ended a different session")
	}

	// Logging out without a session is fine.
	if rec := s.do(request{method: "POST", path: "/api/v1/auth/logout"}); rec.Code != http.StatusNoContent {
		t.Errorf("logout without a cookie: %d", rec.Code)
	}
}

func TestLoginReplacesThisBrowsersSession(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	old := s.setup("admin", "long enough")
	rec := s.do(request{method: "POST", path: "/api/v1/auth/login", body: creds("admin", "long enough"), cookie: old})
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	if resp, _ := s.session(old); resp.User != nil {
		t.Error("the previous session still works after signing in again")
	}
}

// Wrong password and unknown user must look identical to the client.
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	s.setup("alice", "alice password")

	var bodies []string
	for _, c := range []string{
		creds("alice", "wrong password"),
		creds("nobody", "alice password"),
		creds("", ""),
	} {
		rec := s.do(request{method: "POST", path: "/api/v1/auth/login", body: c})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("login %s: %d %s, want 401", c, rec.Code, rec.Body)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Errorf("failed login set a cookie")
		}
		bodies = append(bodies, rec.Body.String())
	}
	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Errorf("failure bodies differ: %q vs %q", bodies[0], b)
		}
	}
	if decodeError(t, httptestRecorder(bodies[0])).Code != "invalid_credentials" {
		t.Errorf("body = %s, want invalid_credentials", bodies[0])
	}
}

func httptestRecorder(body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	rec.Body.WriteString(body)
	return rec
}

func TestLoginRateLimit(t *testing.T) {
	s := newAuthServer(t, authServerOptions{limiter: auth.NewLimiter(2, time.Minute, nil)})
	s.setup("admin", "long enough")
	login := func(password, addr string) *httptest.ResponseRecorder {
		return s.do(request{method: "POST", path: "/api/v1/auth/login", body: creds("admin", password), remoteAddr: addr})
	}

	// Successful logins don't use up attempts.
	for range 3 {
		if rec := login("long enough", "192.0.2.1:1000"); rec.Code != http.StatusOK {
			t.Fatalf("successful login: %d", rec.Code)
		}
	}
	login("wrong", "192.0.2.1:1000")
	login("wrong", "192.0.2.1:1001")
	rec := login("long enough", "192.0.2.1:1002")
	if rec.Code != http.StatusTooManyRequests || decodeError(t, rec).Code != "rate_limited" {
		t.Fatalf("third attempt after two failures: %d %s, want 429 rate_limited", rec.Code, rec.Body)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Errorf("Retry-After = %q, want a positive number of seconds", ra)
	}
	// Other clients aren't affected.
	if rec := login("long enough", "192.0.2.2:1000"); rec.Code != http.StatusOK {
		t.Errorf("another IP: %d, want 200", rec.Code)
	}
	// Addresses in the same IPv6 /64 share a limit.
	login("wrong", "[2001:db8::1]:1000")
	login("wrong", "[2001:db8::2]:1000")
	if rec := login("long enough", "[2001:db8::3]:1000"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("third address in one /64: %d, want 429", rec.Code)
	}
	if rec := login("long enough", "[2001:db8:0:1::1]:1000"); rec.Code != http.StatusOK {
		t.Errorf("another /64: %d, want 200", rec.Code)
	}
}

// Behind a trusted proxy, the limit applies to the real client, not the
// proxy; X-Forwarded-For from anyone else is ignored.
func TestLoginRateLimitBehindProxy(t *testing.T) {
	proxy := netip.MustParsePrefix("10.0.0.1/32")
	s := newAuthServer(t, authServerOptions{limiter: auth.NewLimiter(1, time.Minute, nil), proxies: []netip.Prefix{proxy}})
	s.setup("admin", "long enough")
	login := func(addr, xff string) int {
		return s.do(request{
			method: "POST", path: "/api/v1/auth/login", body: creds("admin", "wrong"),
			remoteAddr: addr, header: map[string]string{"X-Forwarded-For": xff},
		}).Code
	}
	if code := login("10.0.0.1:1", "192.0.2.1"); code != http.StatusUnauthorized {
		t.Fatalf("first client via proxy: %d", code)
	}
	if code := login("10.0.0.1:1", "192.0.2.2"); code != http.StatusUnauthorized {
		t.Errorf("second client via the same proxy: %d, want 401 (its own limit)", code)
	}
	if code := login("10.0.0.1:1", "192.0.2.1"); code != http.StatusTooManyRequests {
		t.Errorf("first client again: %d, want 429", code)
	}
	// An untrusted peer can't dodge the limit by inventing X-Forwarded-For.
	if code := login("198.51.100.7:1", "192.0.2.50"); code != http.StatusUnauthorized {
		t.Fatalf("direct client: %d", code)
	}
	if code := login("198.51.100.7:1", "192.0.2.51"); code != http.StatusTooManyRequests {
		t.Errorf("direct client with a new X-Forwarded-For: %d, want 429", code)
	}
}

func TestSessionRenewalRefreshesCookie(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	c := s.setup("admin", "long enough")

	// Same day: no new cookie.
	s.clock.t = s.clock.t.Add(time.Hour)
	if _, rec := s.session(c); len(rec.Result().Cookies()) != 0 {
		t.Errorf("an hour later: Set-Cookie %q, want none", rec.Header().Values("Set-Cookie"))
	}
	// After a day the cookie is re-sent with the new expiry.
	s.clock.t = s.clock.t.Add(48 * time.Hour)
	_, rec := s.session(c)
	renewed := sessionCookieFrom(t, rec)
	if renewed.Value != c.Value || !renewed.Expires.Equal(s.clock.Now().Add(auth.DefaultSessionTTL)) {
		t.Errorf("renewed cookie = %+v, want the same token expiring 30 days from now", renewed)
	}
	if !renewed.HttpOnly || renewed.SameSite != http.SameSiteLaxMode {
		t.Errorf("renewed cookie lost its flags: %+v", renewed)
	}
}

func TestSecureCookie(t *testing.T) {
	proxy := netip.MustParsePrefix("10.0.0.0/8")
	tests := []struct {
		name   string
		opts   authServerOptions
		header map[string]string
		addr   string
		secure bool
	}{
		{name: "plain HTTP", secure: false},
		{name: "https base URL", opts: authServerOptions{baseOrigin: "https://watcht.example"}, secure: true},
		{name: "http base URL", opts: authServerOptions{baseOrigin: "http://watcht.lan"}, secure: false},
		{
			name: "trusted proxy says https", opts: authServerOptions{proxies: []netip.Prefix{proxy}},
			header: map[string]string{"X-Forwarded-Proto": "https"}, addr: "10.1.2.3:4", secure: true,
		},
		{
			name: "untrusted peer says https", opts: authServerOptions{proxies: []netip.Prefix{proxy}},
			header: map[string]string{"X-Forwarded-Proto": "https"}, addr: "192.0.2.1:4", secure: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newAuthServer(t, tt.opts)
			h := map[string]string{}
			for k, v := range tt.header {
				h[k] = v
			}
			if tt.opts.baseOrigin != "" {
				h["Origin"] = tt.opts.baseOrigin
			}
			rec := s.do(request{method: "POST", path: "/api/v1/auth/setup", body: creds("admin", "long enough"), header: h, remoteAddr: tt.addr})
			if rec.Code != http.StatusCreated {
				t.Fatalf("setup: %d %s", rec.Code, rec.Body)
			}
			if c := sessionCookieFrom(t, rec); c.Secure != tt.secure {
				t.Errorf("Secure = %v, want %v", c.Secure, tt.secure)
			}
		})
	}
}

func TestCrossOriginRequestsAreBlocked(t *testing.T) {
	s := newAuthServer(t, authServerOptions{baseOrigin: "https://watcht.example"})
	c := s.setup("admin", "long enough")

	for _, path := range []string{"/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/auth/setup"} {
		rec := s.do(request{method: "POST", path: path, body: creds("admin", "long enough"), cookie: c, crossSite: true})
		if rec.Code != http.StatusForbidden || decodeError(t, rec).Code != "cross_origin" {
			t.Errorf("cross-site POST %s: %d %s, want 403 cross_origin", path, rec.Code, rec.Body)
		}
	}
	if resp, _ := s.session(c); resp.User == nil {
		t.Error("a blocked cross-site logout still ended the session")
	}

	// Older browsers without Sec-Fetch-Site: Origin is compared with Host.
	rec := s.do(request{method: "POST", path: "/api/v1/auth/logout", cookie: c, header: map[string]string{
		"Sec-Fetch-Site": "", "Origin": "https://evil.example",
	}})
	if rec.Code != http.StatusForbidden {
		t.Errorf("foreign Origin without Sec-Fetch-Site: %d, want 403", rec.Code)
	}
	// A proxy that rewrites Host: the configured base URL is still trusted.
	rec = s.do(request{method: "POST", path: "/api/v1/auth/login", body: creds("admin", "long enough"), header: map[string]string{
		"Sec-Fetch-Site": "", "Origin": "https://watcht.example",
	}})
	if rec.Code != http.StatusOK {
		t.Errorf("Origin = WATCHT_BASE_URL with another Host: %d %s, want 200", rec.Code, rec.Body)
	}
	// Non-browser clients send neither header and aren't subject to CSRF.
	rec = s.do(request{method: "POST", path: "/api/v1/auth/login", body: creds("admin", "long enough"), header: map[string]string{"Sec-Fetch-Site": ""}})
	if rec.Code != http.StatusOK {
		t.Errorf("request without browser headers: %d %s, want 200", rec.Code, rec.Body)
	}
	// Reads aren't blocked.
	rec = s.do(request{method: "GET", path: "/api/v1/auth/session", cookie: c, crossSite: true})
	if rec.Code != http.StatusOK {
		t.Errorf("cross-site GET: %d, want 200", rec.Code)
	}
}

func TestAPIRequestValidation(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	tests := []struct {
		name        string
		body        string
		contentType string
		status      int
		code        string
	}{
		{"form body", "username=a&password=b", "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"no content type", creds("a", "b"), "", http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"JSON with charset", creds("admin", "long enough"), "application/json; charset=utf-8", http.StatusCreated, ""},
		{"malformed", `{"username":`, "application/json", http.StatusBadRequest, "invalid_json"},
		{"unknown field", `{"username":"a","password":"b","is_admin":true}`, "application/json", http.StatusBadRequest, "invalid_json"},
		{"two values", creds("a", "b") + creds("a", "b"), "application/json", http.StatusBadRequest, "invalid_json"},
		{"too large", `{"username":"` + strings.Repeat("a", maxBodyBytes) + `"}`, "application/json", http.StatusRequestEntityTooLarge, "too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := s.do(request{method: "POST", path: "/api/v1/auth/setup", body: tt.body, header: map[string]string{"Content-Type": tt.contentType}})
			if rec.Code != tt.status {
				t.Fatalf("status = %d (%s), want %d", rec.Code, rec.Body, tt.status)
			}
			if tt.code != "" && decodeError(t, rec).Code != tt.code {
				t.Errorf("code = %s, want %s", decodeError(t, rec).Code, tt.code)
			}
		})
	}
}

func TestAPIWrongMethodIs405(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	rec := s.do(request{method: "GET", path: "/api/v1/auth/login"})
	if rec.Code != http.StatusMethodNotAllowed || decodeError(t, rec).Code != "method_not_allowed" {
		t.Fatalf("GET login: %d %s, want 405 method_not_allowed", rec.Code, rec.Body)
	}
	if allow := rec.Header().Get("Allow"); allow != "POST" {
		t.Errorf("Allow = %q, want POST", allow)
	}
	rec = s.do(request{method: "DELETE", path: "/api/v1/auth/session"})
	if rec.Code != http.StatusMethodNotAllowed || !strings.Contains(rec.Header().Get("Allow"), "GET") {
		t.Errorf("DELETE session: %d, Allow %q; want 405 allowing GET", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestAuthLogsNoSecrets(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	c := s.setup("admin", "a secret password")
	s.do(request{method: "POST", path: "/api/v1/auth/login", body: creds("admin", "another secret"), cookie: c})
	s.session(c)
	s.do(request{method: "POST", path: "/api/v1/auth/logout", cookie: c})
	out := s.logs.String()
	for _, secret := range []string{"a secret password", "another secret", c.Value} {
		if strings.Contains(out, secret) {
			t.Errorf("the log contains %q:\n%s", secret, out)
		}
	}
}
