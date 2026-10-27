package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
)

// tokenPath is the path of one API token, e.g. /api/v1/tokens/7.
func tokenPath(id int64) string {
	return "/api/v1/tokens/" + strconv.FormatInt(id, 10)
}

// bearerHeader is the Authorization header that sends an API token.
func bearerHeader(secret string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + secret}
}

// issueToken creates a token named name as the user behind cookie, who gives
// their current password, and returns its metadata and secret. It fails the
// test unless the token is created.
func (s *authServer) issueToken(cookie *http.Cookie, password, name string) (tokenJSON, string) {
	s.t.Helper()
	rec := s.createTokenAs(cookie, password, name)
	if rec.Code != http.StatusCreated {
		s.t.Fatalf("create token %q: %d %s", name, rec.Code, rec.Body)
	}
	var created struct {
		Token  tokenJSON `json:"token"`
		Secret string    `json:"secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		s.t.Fatal(err)
	}
	return created.Token, created.Secret
}

// createTokenAs sends POST /api/v1/tokens as the user behind cookie, with
// their current password.
func (s *authServer) createTokenAs(cookie *http.Cookie, password, name string) *httptest.ResponseRecorder {
	return s.do(request{
		method: http.MethodPost, path: "/api/v1/tokens", cookie: cookie,
		body: jsonBody(s.t, map[string]string{"name": name, "password": password}),
	})
}

// tokensOf returns GET /api/v1/tokens as the user behind cookie sees it.
func (s *authServer) tokensOf(cookie *http.Cookie) []tokenJSON {
	s.t.Helper()
	rec := s.do(request{method: http.MethodGet, path: "/api/v1/tokens", cookie: cookie})
	if rec.Code != http.StatusOK {
		s.t.Fatalf("list tokens: %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Tokens []tokenJSON `json:"tokens"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		s.t.Fatal(err)
	}
	return resp.Tokens
}

// revokeTokenAs sends DELETE /api/v1/tokens/{id} as the user behind cookie.
func (s *authServer) revokeTokenAs(cookie *http.Cookie, id int64) *httptest.ResponseRecorder {
	return s.do(request{method: http.MethodDelete, path: tokenPath(id), cookie: cookie})
}

// meRequest sends GET /api/v1/me with the credentials in req.
func (s *authServer) meRequest(req request) *httptest.ResponseRecorder {
	req.method, req.path = http.MethodGet, "/api/v1/me"
	return s.do(req)
}

// userOf returns the user behind a session cookie.
func (s *authServer) userOf(cookie *http.Cookie) userJSON {
	s.t.Helper()
	resp, _ := s.session(cookie)
	if resp.User == nil {
		s.t.Fatal("the cookie isn't signed in")
	}
	return *resp.User
}

// expectMe checks that rec is a 200 from GET /api/v1/me for want.
func expectMe(t *testing.T, rec *httptest.ResponseRecorder, want userJSON) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /me: %d %s, want 200", rec.Code, rec.Body)
	}
	var resp struct {
		User *userJSON `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.User == nil {
		t.Fatalf("GET /me body = %s", rec.Body)
	}
	if *resp.User != want {
		t.Errorf("GET /me as %+v, want %+v", *resp.User, want)
	}
}

// expectInvalidToken checks that rec refuses an unknown, revoked, or
// deleted token: 401 with the invalid_token challenge.
func expectInvalidToken(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	expectError(t, rec, http.StatusUnauthorized, "unauthenticated")
	if got := rec.Header().Get("WWW-Authenticate"); got != `Bearer error="invalid_token"` {
		t.Errorf("WWW-Authenticate = %q, want Bearer error=\"invalid_token\"", got)
	}
}

// tokenNames lists the names of tokens, in the order given.
func tokenNames(tokens []tokenJSON) []string {
	names := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		names = append(names, tok.Name)
	}
	return names
}

// fieldNames returns the sorted keys of a JSON object, to pin a response's
// shape.
func fieldNames(t *testing.T, object []byte) []string {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(object, &fields); err != nil {
		t.Fatalf("not a JSON object: %s", object)
	}
	return slices.Sorted(maps.Keys(fields))
}

// GET /api/v1/me with a session cookie returns the signed-in user, in the
// documented shape.
func TestMeWithSessionCookie(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	resp, _ := s.session(admin)
	want := userJSON{ID: resp.User.ID, Username: "admin", IsAdmin: true}

	rec := s.meRequest(request{cookie: admin})
	expectMe(t, rec, want)
	if got := fieldNames(t, rec.Body.Bytes()); !slices.Equal(got, []string{"user"}) {
		t.Errorf("top-level fields = %v, want [user]", got)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := fieldNames(t, body["user"]); !slices.Equal(got, []string{"id", "is_admin", "username"}) {
		t.Errorf("user fields = %v, want [id is_admin username]", got)
	}
}

// A bearer token acts as its user. The scheme is case-insensitive, and
// extra spaces before the token are ignored.
func TestMeWithAPIToken(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	s.createAccount(admin, "member", "member password", false)
	memberCookie := s.login("member", "member password")
	_, secret := s.issueToken(memberCookie, "member password", "laptop")
	want := s.userOf(memberCookie)

	for _, tt := range []struct{ name, header string }{
		{"Bearer", "Bearer " + secret},
		{"lowercase scheme", "bearer " + secret},
		{"uppercase scheme", "BEARER " + secret},
		{"extra spaces", "Bearer  " + secret},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expectMe(t, s.meRequest(request{header: map[string]string{"Authorization": tt.header}}), want)
		})
	}
	if want.IsAdmin {
		t.Error("a member's token acted as an admin")
	}
}

// Without credentials, or with a stale cookie, /me is 401.
func TestMeWithoutCredentials(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	s.setup("admin", "admin password")
	for _, tt := range []struct {
		name string
		req  request
	}{
		{"no credentials", request{}},
		{"stale cookie", request{cookie: &http.Cookie{Name: sessionCookie, Value: "stale"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expectError(t, s.meRequest(tt.req), http.StatusUnauthorized, "unauthenticated")
		})
	}
}

// Unknown, revoked, and malformed tokens are all the same 401.
func TestMeRejectsUnknownAndRevokedTokens(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	tok, revoked := s.issueToken(admin, "admin password", "old")
	expectMe(t, s.meRequest(request{header: bearerHeader(revoked)}), s.userOf(admin))
	if rec := s.revokeTokenAs(admin, tok.ID); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
	}

	for name, header := range map[string]string{
		"revoked":        "Bearer " + revoked,
		"unknown":        "Bearer wt_" + strings.Repeat("A", 43),
		"doubled prefix": "Bearer wt_" + revoked,
		"wrong length":   "Bearer wt_short",
	} {
		t.Run(name, func(t *testing.T) {
			expectInvalidToken(t, s.meRequest(request{header: map[string]string{"Authorization": header}}))
		})
	}
}

// The cookie is never a fallback for a watcht token: a bad token is refused
// even with a valid session, and a valid token wins over any cookie. Other
// Authorization headers aren't tokens at all; see
// TestOnlyWatchtTokensSwitchToTokenAuth.
func TestBadTokenNeverFallsBackToCookie(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	s.createAccount(admin, "member", "member password", false)
	memberCookie := s.login("member", "member password")
	tok, revoked := s.issueToken(memberCookie, "member password", "old")
	if rec := s.revokeTokenAs(memberCookie, tok.ID); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
	}
	_, memberSecret := s.issueToken(memberCookie, "member password", "phone")
	_, adminSecret := s.issueToken(admin, "admin password", "ci")
	stale := &http.Cookie{Name: sessionCookie, Value: "stale"}

	for name, header := range map[string]string{
		"unknown token": "Bearer wt_" + strings.Repeat("A", 43),
		"revoked token": "Bearer " + revoked,
	} {
		h := map[string]string{"Authorization": header}
		t.Run(name+" with a valid session", func(t *testing.T) {
			expectError(t, s.meRequest(request{cookie: admin, header: h}), http.StatusUnauthorized, "unauthenticated")
			// Session endpoints refuse a token outright, bad or not, before
			// the cookie is looked at.
			for _, req := range []request{
				{method: http.MethodGet, path: "/api/v1/tokens"},
				{method: http.MethodGet, path: "/api/v1/users"},
				{method: http.MethodPut, path: "/api/v1/me/password", body: jsonBody(t, map[string]string{"current_password": "admin password", "new_password": "new admin password"})},
			} {
				req.cookie, req.header = admin, h
				expectError(t, s.do(req), http.StatusForbidden, "session_required")
			}
		})
	}
	if !s.signedIn(admin) {
		t.Error("the refused requests ended the admin's session")
	}

	expectMe(t, s.meRequest(request{cookie: admin, header: bearerHeader(memberSecret)}), s.userOf(memberCookie))
	expectMe(t, s.meRequest(request{cookie: stale, header: bearerHeader(adminSecret)}), s.userOf(admin))
}

// Token endpoints need a browser session. A token, valid or not, gets 403
// session_required, even with the right password, and no request creates or
// revokes anything.
func TestTokenEndpointsNeedASession(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	ci, ciSecret := s.issueToken(admin, "admin password", "ci")
	backup, _ := s.issueToken(admin, "admin password", "backup")
	stale := &http.Cookie{Name: sessionCookie, Value: "stale"}
	create := jsonBody(t, map[string]string{"name": "intruder", "password": "admin password"})
	unknown := "wt_" + strings.Repeat("A", 43)

	for _, tt := range []struct {
		name   string
		req    request
		status int
		code   string
	}{
		{"list, no credentials", request{method: http.MethodGet, path: "/api/v1/tokens"}, http.StatusUnauthorized, "unauthenticated"},
		{"create, no credentials", request{method: http.MethodPost, path: "/api/v1/tokens", body: create}, http.StatusUnauthorized, "unauthenticated"},
		{"revoke, no credentials", request{method: http.MethodDelete, path: tokenPath(backup.ID)}, http.StatusUnauthorized, "unauthenticated"},
		{"list, stale cookie", request{method: http.MethodGet, path: "/api/v1/tokens", cookie: stale}, http.StatusUnauthorized, "unauthenticated"},
		{"list, API token", request{method: http.MethodGet, path: "/api/v1/tokens", header: bearerHeader(ciSecret)}, http.StatusForbidden, "session_required"},
		{"create, API token", request{method: http.MethodPost, path: "/api/v1/tokens", body: create, header: bearerHeader(ciSecret)}, http.StatusForbidden, "session_required"},
		{"create, unknown API token", request{method: http.MethodPost, path: "/api/v1/tokens", body: create, header: bearerHeader(unknown)}, http.StatusForbidden, "session_required"},
		{"revoke another token, API token", request{method: http.MethodDelete, path: tokenPath(backup.ID), header: bearerHeader(ciSecret)}, http.StatusForbidden, "session_required"},
		{"revoke its own token, API token", request{method: http.MethodDelete, path: tokenPath(ci.ID), header: bearerHeader(ciSecret)}, http.StatusForbidden, "session_required"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expectError(t, s.do(tt.req), tt.status, tt.code)
		})
	}

	if got := tokenNames(s.tokensOf(admin)); !slices.Equal(got, []string{"ci", "backup"}) {
		t.Errorf("tokens = %v, want [ci backup] (nothing created or revoked)", got)
	}
	expectMe(t, s.meRequest(request{header: bearerHeader(ciSecret)}), s.userOf(admin))
}

// Creating a token returns its secret once, with the metadata. The token
// is stored only as its SHA-256, and it works on /me.
func TestCreateToken(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")

	rec := s.createTokenAs(admin, "admin password", "  jellyfin  ")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if got := fieldNames(t, rec.Body.Bytes()); !slices.Equal(got, []string{"secret", "token"}) {
		t.Errorf("top-level fields = %v, want [secret token]", got)
	}
	var body struct {
		Token  tokenJSON `json:"token"`
		Secret string    `json:"secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if got := fieldNames(t, raw["token"]); !slices.Equal(got, []string{"created_at", "id", "name"}) {
		t.Errorf("token fields = %v, want [created_at id name]", got)
	}
	tok, secret := body.Token, body.Secret
	if tok.ID == 0 || tok.Name != "jellyfin" || tok.CreatedAt != s.clock.Now().UTC().Format(time.RFC3339) {
		t.Errorf("token = %+v, want the trimmed name, an ID, and the creation time", tok)
	}
	if !strings.HasPrefix(secret, "wt_") || len(secret) != len("wt_")+43 {
		t.Fatalf("secret has length %d, want wt_ followed by 43 characters", len(secret))
	}

	// Only the SHA-256 of the secret is stored.
	raw64, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(secret, "wt_"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw64)
	var stored int
	if err := s.db.Write.QueryRow("SELECT COUNT(*) FROM api_tokens WHERE token_hash = ?", sum[:]).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != 1 {
		t.Errorf("rows with the SHA-256 of the secret = %d, want 1", stored)
	}

	expectMe(t, s.meRequest(request{header: bearerHeader(secret)}), s.userOf(admin))
	if got := s.tokensOf(admin); len(got) != 1 || got[0] != tok {
		t.Errorf("list = %+v, want [%+v]", got, tok)
	}
	list := s.do(request{method: http.MethodGet, path: "/api/v1/tokens", cookie: admin})
	if strings.Contains(list.Body.String(), secret) {
		t.Error("the list response contains the secret")
	}
}

// Creating a token needs the current password. A wrong, empty, or missing
// one is invalid_credentials, and no token is created.
func TestCreateTokenNeedsTheCurrentPassword(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")

	for _, body := range []string{
		jsonBody(t, map[string]string{"name": "laptop", "password": "not the password"}),
		jsonBody(t, map[string]string{"name": "laptop", "password": ""}),
		jsonBody(t, map[string]string{"name": "laptop"}),
	} {
		rec := s.do(request{method: http.MethodPost, path: "/api/v1/tokens", cookie: admin, body: body})
		expectError(t, rec, http.StatusBadRequest, "invalid_credentials")
	}
	if got := s.tokensOf(admin); len(got) != 0 {
		t.Errorf("tokens = %v, want none after the refused creations", tokenNames(got))
	}
	s.issueToken(admin, "admin password", "laptop")
	if got := tokenNames(s.tokensOf(admin)); !slices.Equal(got, []string{"laptop"}) {
		t.Errorf("tokens = %v, want [laptop]", got)
	}
}

// A wrong current password uses up the account's attempts, as it does for a
// password change. Over the limit, even the right password gets 429.
func TestCreateTokenRateLimitPerAccount(t *testing.T) {
	s := newAuthServer(t, authServerOptions{limiter: auth.NewLimiter(2, time.Hour, nil)})
	admin := s.setup("admin", "admin password")

	expectError(t, s.createTokenAs(admin, "wrong guess", "laptop"), http.StatusBadRequest, "invalid_credentials")
	expectError(t, s.createTokenAs(admin, "wrong guess", "laptop"), http.StatusBadRequest, "invalid_credentials")
	rec := s.createTokenAs(admin, "admin password", "laptop")
	expectError(t, rec, http.StatusTooManyRequests, "rate_limited")
	if ra := rec.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Errorf("Retry-After = %q, want a positive number of seconds", ra)
	}
	if got := s.tokensOf(admin); len(got) != 0 {
		t.Errorf("tokens = %v, want none", tokenNames(got))
	}
}

// Creating a token and changing the password share one limit per account:
// a wrong guess at each uses up a burst of two.
func TestTokenAndPasswordChangeShareTheLimit(t *testing.T) {
	s := newAuthServer(t, authServerOptions{limiter: auth.NewLimiter(2, time.Hour, nil)})
	admin := s.setup("admin", "admin password")

	expectError(t, s.createTokenAs(admin, "wrong guess", "laptop"), http.StatusBadRequest, "invalid_credentials")
	expectError(t, s.changeOwnPassword(admin, "wrong guess", "new admin password"), http.StatusBadRequest, "invalid_credentials")
	expectError(t, s.createTokenAs(admin, "admin password", "laptop"), http.StatusTooManyRequests, "rate_limited")
	expectError(t, s.changeOwnPassword(admin, "admin password", "new admin password"), http.StatusTooManyRequests, "rate_limited")
	// The refused change left the password alone.
	s.login("admin", "admin password")
	if got := s.tokensOf(admin); len(got) != 0 {
		t.Errorf("tokens = %v, want none", tokenNames(got))
	}
}

// A successful creation gives its attempt back, so with one attempt allowed
// two creations in a row both work.
func TestSuccessfulTokenCreationDoesNotUseAnAttempt(t *testing.T) {
	s := newAuthServer(t, authServerOptions{limiter: auth.NewLimiter(1, time.Hour, nil)})
	admin := s.setup("admin", "admin password")

	s.issueToken(admin, "admin password", "laptop")
	s.issueToken(admin, "admin password", "phone")
	// One wrong guess uses that single attempt, so the next try waits.
	expectError(t, s.createTokenAs(admin, "wrong guess", "tablet"), http.StatusBadRequest, "invalid_credentials")
	expectError(t, s.createTokenAs(admin, "admin password", "tablet"), http.StatusTooManyRequests, "rate_limited")
	if got := tokenNames(s.tokensOf(admin)); !slices.Equal(got, []string{"laptop", "phone"}) {
		t.Errorf("tokens = %v, want [laptop phone]", got)
	}
}

// A name that's invalid, or already taken, doesn't use up an attempt. The
// right password is no wrong guess either, so with one attempt allowed these
// can be retried as often as needed.
func TestInvalidOrTakenNameDoesNotUseAnAttempt(t *testing.T) {
	s := newAuthServer(t, authServerOptions{limiter: auth.NewLimiter(1, time.Hour, nil)})
	admin := s.setup("admin", "admin password")
	s.issueToken(admin, "admin password", "laptop")

	for range 3 {
		expectError(t, s.createTokenAs(admin, "admin password", "   "), http.StatusBadRequest, "invalid_input")
	}
	for range 3 {
		expectError(t, s.createTokenAs(admin, "admin password", "laptop"), http.StatusConflict, "name_taken")
	}
	s.issueToken(admin, "admin password", "phone")
	if got := tokenNames(s.tokensOf(admin)); !slices.Equal(got, []string{"laptop", "phone"}) {
		t.Errorf("tokens = %v, want [laptop phone]", got)
	}
}

// Names are checked after trimming: blank, control characters, and more
// than 64 characters are invalid_input, and 64 characters is fine.
func TestTokenNames(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")

	for _, tt := range []struct {
		name string
		body string
	}{
		{"empty", jsonBody(t, map[string]string{"name": "", "password": "admin password"})},
		{"missing", `{"password":"admin password"}`},
		{"only spaces", jsonBody(t, map[string]string{"name": "   ", "password": "admin password"})},
		{"only a tab", jsonBody(t, map[string]string{"name": "\t", "password": "admin password"})},
		{"NUL character", jsonBody(t, map[string]string{"name": "a\x00b", "password": "admin password"})},
		{"newline", jsonBody(t, map[string]string{"name": "two\nlines", "password": "admin password"})},
		{"65 characters", jsonBody(t, map[string]string{"name": strings.Repeat("n", 65), "password": "admin password"})},
		{"65 accented characters", jsonBody(t, map[string]string{"name": strings.Repeat("é", 65), "password": "admin password"})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := s.do(request{method: http.MethodPost, path: "/api/v1/tokens", cookie: admin, body: tt.body})
			expectError(t, rec, http.StatusBadRequest, "invalid_input")
		})
	}

	accepted := []string{strings.Repeat("n", 64), strings.Repeat("é", 64)}
	for i, name := range accepted {
		t.Run("64 characters "+strconv.Itoa(i+1), func(t *testing.T) {
			tok, _ := s.issueToken(admin, "admin password", name)
			if tok.Name != name {
				t.Errorf("name = %q, want it kept as sent", tok.Name)
			}
		})
	}
	if got := tokenNames(s.tokensOf(admin)); !slices.Equal(got, accepted) {
		t.Errorf("tokens = %v, want only the two 64-character names", got)
	}
}

// A name is unique per user, after trimming. A member can use the same
// name as the admin, and a revoked name can be used again.
func TestTokenNamesAreUniquePerUser(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	s.createAccount(admin, "member", "member password", false)
	memberCookie := s.login("member", "member password")

	tok, _ := s.issueToken(admin, "admin password", "jellyfin")
	for _, name := range []string{"  jellyfin  ", "jellyfin"} {
		expectError(t, s.createTokenAs(admin, "admin password", name), http.StatusConflict, "name_taken")
	}
	if got := tokenNames(s.tokensOf(admin)); !slices.Equal(got, []string{"jellyfin"}) {
		t.Errorf("admin tokens = %v, want [jellyfin]", got)
	}

	memberTok, _ := s.issueToken(memberCookie, "member password", "jellyfin")
	if got := s.tokensOf(memberCookie); len(got) != 1 || got[0] != memberTok {
		t.Errorf("member tokens = %+v, want [%+v]", got, memberTok)
	}

	if rec := s.revokeTokenAs(admin, tok.ID); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
	}
	again, _ := s.issueToken(admin, "admin password", "jellyfin")
	if got := s.tokensOf(admin); len(got) != 1 || got[0] != again {
		t.Errorf("admin tokens = %+v, want [%+v]", got, again)
	}
}

// Revoking a token stops it at once. A second revoke, or an ID that isn't
// one of the user's tokens, is 404 not_found.
func TestRevokeToken(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	tok, secret := s.issueToken(admin, "admin password", "laptop")
	other, otherSecret := s.issueToken(admin, "admin password", "phone")

	rec := s.revokeTokenAs(admin, tok.ID)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("revoke: %d %s, want 204 with no body", rec.Code, rec.Body)
	}
	expectInvalidToken(t, s.meRequest(request{header: bearerHeader(secret)}))

	expectError(t, s.revokeTokenAs(admin, tok.ID), http.StatusNotFound, "not_found")
	for _, p := range []string{tokenPath(99999), "/api/v1/tokens/abc", "/api/v1/tokens/0"} {
		expectError(t, s.do(request{method: http.MethodDelete, path: p, cookie: admin}), http.StatusNotFound, "not_found")
	}

	if got := s.tokensOf(admin); len(got) != 1 || got[0] != other {
		t.Errorf("tokens = %+v, want only [%+v]", got, other)
	}
	expectMe(t, s.meRequest(request{header: bearerHeader(otherSecret)}), s.userOf(admin))
}

// Tokens belong to their user. A member's token acts as the member, and
// each user's list shows only their own tokens. Neither can revoke the
// other's, not even an admin's token.
func TestTokensArePerUser(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	s.createAccount(admin, "member", "member password", false)
	memberCookie := s.login("member", "member password")
	adminTok, adminSecret := s.issueToken(admin, "admin password", "admin-ci")
	memberTok, memberSecret := s.issueToken(memberCookie, "member password", "member-ci")
	adminUser, memberUser := s.userOf(admin), s.userOf(memberCookie)

	expectMe(t, s.meRequest(request{header: bearerHeader(memberSecret)}), memberUser)
	expectMe(t, s.meRequest(request{header: bearerHeader(adminSecret)}), adminUser)
	if memberUser.IsAdmin {
		t.Fatal("the member is an admin")
	}
	if got := tokenNames(s.tokensOf(admin)); !slices.Equal(got, []string{"admin-ci"}) {
		t.Errorf("admin's tokens = %v, want [admin-ci]", got)
	}
	if got := tokenNames(s.tokensOf(memberCookie)); !slices.Equal(got, []string{"member-ci"}) {
		t.Errorf("member's tokens = %v, want [member-ci]", got)
	}

	expectError(t, s.revokeTokenAs(admin, memberTok.ID), http.StatusNotFound, "not_found")
	expectError(t, s.revokeTokenAs(memberCookie, adminTok.ID), http.StatusNotFound, "not_found")
	expectMe(t, s.meRequest(request{header: bearerHeader(memberSecret)}), memberUser)
	expectMe(t, s.meRequest(request{header: bearerHeader(adminSecret)}), adminUser)

	// An admin's token is still an API token: it can't manage accounts.
	expectError(t, s.do(request{method: http.MethodGet, path: "/api/v1/users", header: bearerHeader(adminSecret)}), http.StatusForbidden, "session_required")
}

// Deleting a user revokes their tokens. A new account with the same name
// doesn't inherit them.
func TestDeletingAUserRevokesTheirTokens(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	member := s.createAccount(admin, "member", "member password", false)
	memberCookie := s.login("member", "member password")
	_, memberSecret := s.issueToken(memberCookie, "member password", "ci")
	_, adminSecret := s.issueToken(admin, "admin password", "ci")
	expectMe(t, s.meRequest(request{header: bearerHeader(memberSecret)}), s.userOf(memberCookie))

	if rec := s.deleteUserAs(admin, member.ID); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	expectInvalidToken(t, s.meRequest(request{header: bearerHeader(memberSecret)}))
	expectMe(t, s.meRequest(request{header: bearerHeader(adminSecret)}), s.userOf(admin))

	newcomer := s.createAccount(admin, "member", "new member password", false)
	if newcomer.ID == member.ID {
		t.Fatalf("the new account reused the deleted account's ID %d", member.ID)
	}
	expectInvalidToken(t, s.meRequest(request{header: bearerHeader(memberSecret)}))
	if got := s.tokensOf(s.login("member", "new member password")); len(got) != 0 {
		t.Errorf("the new account's tokens = %v, want none", tokenNames(got))
	}
}

// Cross-site POST and DELETE on the token endpoints are refused before they
// run, with a cookie. Reads aren't blocked.
func TestCrossSiteTokenRequestsAreBlocked(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	tok, secret := s.issueToken(admin, "admin password", "ci")
	create := jsonBody(t, map[string]string{"name": "evil", "password": "admin password"})

	for _, req := range []request{
		{method: http.MethodPost, path: "/api/v1/tokens", body: create},
		{method: http.MethodDelete, path: tokenPath(tok.ID)},
	} {
		req.cookie, req.crossSite = admin, true
		t.Run(req.method+" "+req.path, func(t *testing.T) {
			expectError(t, s.do(req), http.StatusForbidden, "cross_origin")
		})
	}
	// An old browser without Sec-Fetch-Site: a foreign Origin is refused.
	rec := s.do(request{method: http.MethodPost, path: "/api/v1/tokens", cookie: admin, body: create, header: map[string]string{
		"Sec-Fetch-Site": "", "Origin": "https://evil.example",
	}})
	expectError(t, rec, http.StatusForbidden, "cross_origin")

	if got := tokenNames(s.tokensOf(admin)); !slices.Equal(got, []string{"ci"}) {
		t.Errorf("tokens = %v, want [ci] (nothing created or revoked)", got)
	}
	expectMe(t, s.meRequest(request{header: bearerHeader(secret)}), s.userOf(admin))
	if rec := s.do(request{method: http.MethodGet, path: "/api/v1/tokens", cookie: admin, crossSite: true}); rec.Code != http.StatusOK {
		t.Errorf("cross-site GET: %d, want 200", rec.Code)
	}
}

// A script sends no browser headers, so the cross-origin check lets its
// token through. The token still can't manage tokens: the same request
// gets 403 session_required, not cross_origin.
func TestScriptRequestsWithATokenAreNotCrossSite(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	tok, secret := s.issueToken(admin, "admin password", "script")
	script := map[string]string{"Sec-Fetch-Site": "", "Authorization": "Bearer " + secret}

	expectMe(t, s.meRequest(request{header: script}), s.userOf(admin))
	expectError(t, s.do(request{method: http.MethodPost, path: "/api/v1/tokens", header: script, body: jsonBody(t, map[string]string{"name": "more", "password": "admin password"})}), http.StatusForbidden, "session_required")
	expectError(t, s.do(request{method: http.MethodDelete, path: tokenPath(tok.ID), header: script}), http.StatusForbidden, "session_required")

	if got := tokenNames(s.tokensOf(admin)); !slices.Equal(got, []string{"script"}) {
		t.Errorf("tokens = %v, want [script]", got)
	}
	expectMe(t, s.meRequest(request{header: script}), s.userOf(admin))
}

// The secret is never logged, whichever request carries it. The log has
// request lines, so the check isn't vacuous.
func TestTokenSecretIsNeverLogged(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	tok, secret := s.issueToken(admin, "admin password", "ci")

	for _, req := range []request{
		{method: http.MethodGet, path: "/api/v1/me", header: bearerHeader(secret)},
		{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Authorization": "Bearer " + secret + "x"}},
		{method: http.MethodGet, path: "/api/v1/me", header: map[string]string{"Authorization": "Basic " + secret}},
		{method: http.MethodGet, path: "/api/v1/tokens", header: bearerHeader(secret)},
		{method: http.MethodPost, path: "/api/v1/tokens", header: bearerHeader(secret), body: jsonBody(t, map[string]string{"name": "more", "password": "admin password"})},
		{method: http.MethodGet, path: "/api/v1/tokens", cookie: admin},
		{method: http.MethodDelete, path: tokenPath(tok.ID), cookie: admin},
		{method: http.MethodGet, path: "/api/v1/me", header: bearerHeader(secret)},
	} {
		s.do(req)
	}

	logs := s.logs.String()
	if !strings.Contains(logs, "path=/api/v1/me") {
		t.Fatalf("the log has no request lines to check:\n%s", logs)
	}
	// The log itself isn't printed on failure: it would contain the secret.
	for _, leak := range []string{secret, strings.TrimPrefix(secret, "wt_"), "Bearer " + secret} {
		if strings.Contains(logs, leak) {
			t.Error("the log contains a token secret")
		}
	}
}

// The password is never in a response or the log, whether the creation
// succeeds, fails, or is refused. The passwords are distinctive, so a leak
// can't pass unnoticed.
func TestTokenPasswordIsNeverEchoedOrLogged(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "Sekrit-admin-1")

	var bodies []string
	for _, body := range []string{
		jsonBody(t, map[string]string{"name": "laptop", "password": "Sekrit-guess-2"}),
		jsonBody(t, map[string]string{"name": "laptop"}),
		jsonBody(t, map[string]string{"name": "laptop", "password": "Sekrit-admin-1"}),
		jsonBody(t, map[string]string{"name": "laptop", "password": "Sekrit-admin-1"}),
		jsonBody(t, map[string]string{"name": "  ", "password": "Sekrit-admin-1"}),
	} {
		rec := s.do(request{method: http.MethodPost, path: "/api/v1/tokens", cookie: admin, body: body})
		bodies = append(bodies, rec.Body.String())
	}

	logs := s.logs.String()
	if !strings.Contains(logs, "path=/api/v1/tokens") {
		t.Fatal("the log has no request lines to check")
	}
	// The log isn't printed on failure: it would contain the password.
	for _, secret := range []string{"Sekrit-admin-1", "Sekrit-guess-2"} {
		for _, body := range bodies {
			if strings.Contains(body, secret) {
				t.Errorf("a response body contains %q: %s", secret, body)
			}
		}
		if strings.Contains(logs, secret) {
			t.Errorf("the log contains %q", secret)
		}
	}
}

// Only a watcht token (Authorization: Bearer wt_...) makes a request a token
// request. Any other Authorization header is ignored, as if it weren't there,
// so the session cookie decides: a reverse proxy's Basic auth or an
// oauth2-proxy JWT comes with every browser request. The secret is a real
// token, so a header that accepted it would sign in without a cookie.
func TestOnlyWatchtTokensSwitchToTokenAuth(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	_, secret := s.issueToken(admin, "admin password", "ci")
	want := s.userOf(admin)

	for name, value := range map[string]string{
		"empty":                   "",
		"Basic":                   "Basic dXNlcjpwYXNz",
		"Basic x":                 "Basic x",
		"Basic with the secret":   "Basic " + secret,
		"Token scheme":            "Token " + secret,
		"Bearer without a token":  "Bearer",
		"Bearer with only spaces": "Bearer   ",
		"tab instead of a space":  "Bearer\t" + secret,
		"Bearer JWT":              "Bearer eyJhbGciOiJIUzI1NiJ9.e30.c2ln",
		"secret without its wt_":  "Bearer " + strings.TrimPrefix(secret, "wt_"),
	} {
		h := map[string]string{"Authorization": value}
		t.Run(name+" without a cookie", func(t *testing.T) {
			rec := s.meRequest(request{header: h})
			expectError(t, rec, http.StatusUnauthorized, "unauthenticated")
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("WWW-Authenticate = %q, want Bearer", got)
			}
			for _, path := range []string{"/api/v1/tokens", "/api/v1/users"} {
				expectError(t, s.do(request{method: http.MethodGet, path: path, header: h}), http.StatusUnauthorized, "unauthenticated")
			}
		})
		t.Run(name+" with a valid cookie", func(t *testing.T) {
			expectMe(t, s.meRequest(request{cookie: admin, header: h}), want)
			for _, path := range []string{"/api/v1/tokens", "/api/v1/users"} {
				if rec := s.do(request{method: http.MethodGet, path: path, cookie: admin, header: h}); rec.Code != http.StatusOK {
					t.Errorf("GET %s: %d %s, want 200", path, rec.Code, rec.Body)
				}
			}
		})
	}

	// Creating a token works the same way behind a Basic-auth proxy.
	created := s.do(request{
		method: http.MethodPost, path: "/api/v1/tokens", cookie: admin,
		header: map[string]string{"Authorization": "Basic dXNlcjpwYXNz"},
		body:   jsonBody(t, map[string]string{"name": "proxied", "password": "admin password"}),
	})
	if created.Code != http.StatusCreated {
		t.Errorf("POST /tokens with a session and Basic auth: %d %s, want 201", created.Code, created.Body)
	}
}

// Token IDs aren't reused, so revoking from a stale list can't hit a
// newer token.
func TestTokenIDsAreNotReused(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "long enough")
	create := func(name string) int64 {
		rec := s.do(request{method: http.MethodPost, path: "/api/v1/tokens", cookie: admin, body: `{"name":"` + name + `","password":"long enough"}`})
		var out struct{ Token struct{ ID int64 } }
		if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatalf("create %s: %d %s", name, rec.Code, rec.Body)
		}
		return out.Token.ID
	}
	first := create("first")
	if rec := s.do(request{method: http.MethodDelete, path: "/api/v1/tokens/" + strconv.FormatInt(first, 10), cookie: admin}); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: %d", rec.Code)
	}
	if second := create("second"); second <= first {
		t.Errorf("new token got ID %d after revoking %d; IDs must not be reused", second, first)
	}
}
