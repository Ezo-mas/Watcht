package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
)

// jsonBody encodes v as a JSON request body.
func jsonBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// idPath is the path of one account, e.g. /api/v1/users/7/password.
func idPath(id int64, suffix string) string {
	return "/api/v1/users/" + strconv.FormatInt(id, 10) + suffix
}

// login signs in and returns the session cookie. It fails the test if the
// sign-in is refused.
func (s *authServer) login(username, password string) *http.Cookie {
	s.t.Helper()
	rec := s.do(request{method: http.MethodPost, path: "/api/v1/auth/login", body: creds(username, password)})
	if rec.Code != http.StatusOK {
		s.t.Fatalf("login as %s: %d %s", username, rec.Code, rec.Body)
	}
	return sessionCookieFrom(s.t, rec)
}

// expectLoginRefused checks that signing in with these credentials fails.
func (s *authServer) expectLoginRefused(username, password string) {
	s.t.Helper()
	rec := s.do(request{method: http.MethodPost, path: "/api/v1/auth/login", body: creds(username, password)})
	expectError(s.t, rec, http.StatusUnauthorized, "invalid_credentials")
}

// createAccount creates an account as the admin behind cookie. It fails
// the test unless the account is created.
func (s *authServer) createAccount(admin *http.Cookie, username, password string, isAdmin bool) accountJSON {
	s.t.Helper()
	rec := s.do(request{
		method: http.MethodPost, path: "/api/v1/users", cookie: admin,
		body: jsonBody(s.t, map[string]any{"username": username, "password": password, "is_admin": isAdmin}),
	})
	if rec.Code != http.StatusCreated {
		s.t.Fatalf("create %s: %d %s", username, rec.Code, rec.Body)
	}
	var created struct {
		User accountJSON `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		s.t.Fatal(err)
	}
	return created.User
}

// listAccounts returns GET /api/v1/users as the admin behind cookie sees it.
func (s *authServer) listAccounts(admin *http.Cookie) []accountJSON {
	s.t.Helper()
	rec := s.do(request{method: http.MethodGet, path: "/api/v1/users", cookie: admin})
	if rec.Code != http.StatusOK {
		s.t.Fatalf("list users: %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Users []accountJSON `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		s.t.Fatal(err)
	}
	return resp.Users
}

// usernames lists the usernames of accounts, in the order given.
func usernames(accounts []accountJSON) []string {
	names := make([]string, 0, len(accounts))
	for _, a := range accounts {
		names = append(names, a.Username)
	}
	return names
}

// signedIn reports whether cookie belongs to a live session.
func (s *authServer) signedIn(cookie *http.Cookie) bool {
	s.t.Helper()
	resp, _ := s.session(cookie)
	return resp.User != nil
}

// expectError checks that rec is the standard error shape with this status
// and code.
func expectError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	e := decodeError(t, rec)
	if rec.Code != status || e.Code != code {
		t.Errorf("got %d %s, want %d %s (body %s)", rec.Code, e.Code, status, code, rec.Body)
	}
}

// changeOwnPassword sends PUT /api/v1/me/password.
func (s *authServer) changeOwnPassword(cookie *http.Cookie, current, password string) *httptest.ResponseRecorder {
	return s.do(request{
		method: http.MethodPut, path: "/api/v1/me/password", cookie: cookie,
		body: jsonBody(s.t, map[string]string{"current_password": current, "new_password": password}),
	})
}

// resetUserPassword sends PUT /api/v1/users/{id}/password as the admin
// behind cookie.
func (s *authServer) resetUserPassword(admin *http.Cookie, id int64, password string) *httptest.ResponseRecorder {
	return s.do(request{
		method: http.MethodPut, path: idPath(id, "/password"), cookie: admin,
		body: jsonBody(s.t, map[string]string{"password": password}),
	})
}

// deleteUserAs sends DELETE /api/v1/users/{id} as the admin behind cookie.
func (s *authServer) deleteUserAs(admin *http.Cookie, id int64) *httptest.ResponseRecorder {
	return s.do(request{method: http.MethodDelete, path: idPath(id, ""), cookie: admin})
}

// Account endpoints need a signed-in session. A missing or stale cookie is
// 401, and a watcht API token never stands in for one (403
// session_required): API tokens don't reach account management. Other
// Authorization headers (a proxy's Basic auth) are ignored.
func TestAccountEndpointsNeedASession(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	victim := s.createAccount(admin, "victim", "victim password", false)

	routes := []request{
		{method: http.MethodPut, path: "/api/v1/me/password", body: jsonBody(t, map[string]string{"current_password": "admin password", "new_password": "hijacked password"})},
		{method: http.MethodGet, path: "/api/v1/users"},
		{method: http.MethodPost, path: "/api/v1/users", body: jsonBody(t, map[string]any{"username": "intruder", "password": "intruder password", "is_admin": true})},
		{method: http.MethodDelete, path: idPath(victim.ID, "")},
		{method: http.MethodPut, path: idPath(victim.ID, "/password"), body: jsonBody(t, map[string]string{"password": "hijacked password"})},
	}
	credentials := map[string]struct {
		req    request
		status int
		code   string
	}{
		"no cookie":    {request{}, http.StatusUnauthorized, "unauthenticated"},
		"API token":    {request{header: map[string]string{"Authorization": "Bearer wt_whatever"}}, http.StatusForbidden, "session_required"},
		"basic auth":   {request{header: map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}}, http.StatusUnauthorized, "unauthenticated"},
		"other bearer": {request{header: map[string]string{"Authorization": "Bearer eyJhbGciOi"}}, http.StatusUnauthorized, "unauthenticated"},
		"stale cookie": {request{cookie: &http.Cookie{Name: sessionCookie, Value: "stale"}}, http.StatusUnauthorized, "unauthenticated"},
	}
	for _, route := range routes {
		for name, c := range credentials {
			req := route
			req.cookie, req.header = c.req.cookie, c.req.header
			t.Run(route.method+" "+route.path+" with "+name, func(t *testing.T) {
				expectError(t, s.do(req), c.status, c.code)
			})
		}
	}

	// None of the refused requests changed anything.
	if got := usernames(s.listAccounts(admin)); !slices.Equal(got, []string{"admin", "victim"}) {
		t.Errorf("accounts = %v, want [admin victim]", got)
	}
	s.login("victim", "victim password")
	s.login("admin", "admin password")
}

// A member's session gets 403 on every admin endpoint, and the refused
// requests change nothing.
func TestMembersGetForbiddenOnAdminEndpoints(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	resp, _ := s.session(admin)
	adminID := resp.User.ID
	s.createAccount(admin, "member", "member password", false)
	victim := s.createAccount(admin, "victim", "victim password", false)
	member := s.login("member", "member password")

	for _, req := range []request{
		{method: http.MethodGet, path: "/api/v1/users"},
		{method: http.MethodPost, path: "/api/v1/users", body: jsonBody(t, map[string]any{"username": "intruder", "password": "intruder password", "is_admin": true})},
		{method: http.MethodDelete, path: idPath(victim.ID, "")},
		{method: http.MethodPut, path: idPath(victim.ID, "/password"), body: jsonBody(t, map[string]string{"password": "hijacked password"})},
		{method: http.MethodDelete, path: idPath(adminID, "")},
		{method: http.MethodPut, path: idPath(adminID, "/password"), body: jsonBody(t, map[string]string{"password": "hijacked password"})},
	} {
		req.cookie = member
		t.Run(req.method+" "+req.path, func(t *testing.T) {
			expectError(t, s.do(req), http.StatusForbidden, "forbidden")
		})
	}

	if got := usernames(s.listAccounts(admin)); !slices.Equal(got, []string{"admin", "member", "victim"}) {
		t.Errorf("accounts = %v, want [admin member victim]", got)
	}
	s.login("victim", "victim password")
	s.login("admin", "admin password")
	s.login("member", "member password")
	if !s.signedIn(member) {
		t.Error("the member's own session ended")
	}
}

// An admin creates a member, the member signs in, and the list shows both.
func TestAdminCreatesMemberWhoCanSignIn(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")

	rec := s.do(request{
		method: http.MethodPost, path: "/api/v1/users", cookie: admin,
		body: jsonBody(t, map[string]any{"username": "member", "password": "member password", "is_admin": false}),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created struct {
		User accountJSON `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	member := created.User
	// The clock is whole seconds, so created_at is exact: UTC, RFC 3339.
	if member.ID == 0 || member.Username != "member" || member.IsAdmin || member.CreatedAt != s.clock.Now().UTC().Format(time.RFC3339) {
		t.Errorf("created user = %+v", member)
	}

	cookie := s.login("member", "member password")
	if resp, _ := s.session(cookie); resp.User == nil || resp.User.ID != member.ID || resp.User.IsAdmin {
		t.Errorf("signed in as %+v, want member %d", resp.User, member.ID)
	}

	users := s.listAccounts(admin)
	if len(users) != 2 || users[0].Username != "admin" || !users[0].IsAdmin || users[1] != member {
		t.Errorf("accounts = %+v, want admin (admin) then member (not admin)", users)
	}
}

// The list is ordered by username ignoring case.
func TestAccountListIsOrderedByUsernameIgnoringCase(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	for _, name := range []string{"Zoe", "bob", "Alice", "carol"} {
		s.createAccount(admin, name, "some password", false)
	}
	got := usernames(s.listAccounts(admin))
	if want := []string{"admin", "Alice", "bob", "carol", "Zoe"}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestCreateAccountRejectsBadInput(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	s.createAccount(admin, "member", "member password", false)

	for _, tt := range []struct {
		name, body string
		status     int
		code       string
	}{
		{"bad username", creds("has space", "some password"), http.StatusBadRequest, "invalid_input"},
		{"empty username", creds("", "some password"), http.StatusBadRequest, "invalid_input"},
		{"short password", creds("newbie", "short"), http.StatusBadRequest, "invalid_input"},
		{"missing password", `{"username":"newbie"}`, http.StatusBadRequest, "invalid_input"},
		{"taken", creds("member", "some password"), http.StatusConflict, "username_taken"},
		{"taken in another case", creds("MEMBER", "some password"), http.StatusConflict, "username_taken"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := s.do(request{method: http.MethodPost, path: "/api/v1/users", cookie: admin, body: tt.body})
			expectError(t, rec, tt.status, tt.code)
		})
	}
	if got := usernames(s.listAccounts(admin)); !slices.Equal(got, []string{"admin", "member"}) {
		t.Errorf("accounts = %v, want [admin member]", got)
	}
}

// Deleting an account ends its sessions, on every device.
func TestDeleteEndsTheMembersSessions(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	member := s.createAccount(admin, "member", "member password", false)
	laptop := s.login("member", "member password")
	phone := s.login("member", "member password")
	if !s.signedIn(laptop) || !s.signedIn(phone) {
		t.Fatal("the member's sessions don't work before the delete")
	}

	if rec := s.deleteUserAs(admin, member.ID); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if s.signedIn(laptop) || s.signedIn(phone) {
		t.Error("a session of the deleted member still works")
	}
	s.expectLoginRefused("member", "member password")
	if got := usernames(s.listAccounts(admin)); !slices.Equal(got, []string{"admin"}) {
		t.Errorf("accounts = %v, want [admin]", got)
	}
	expectError(t, s.deleteUserAs(admin, member.ID), http.StatusNotFound, "not_found")

	// SQLite can give the next account the deleted account's ID, so the old
	// cookies must still not sign in as the newcomer.
	s.createAccount(admin, "newcomer", "newcomer password", false)
	if s.signedIn(laptop) {
		t.Error("an old cookie signs in as the next account")
	}
}

// An admin can't delete their own account, and an ID that isn't a positive
// integer is not found. "+2" and "%2B2" must not mean account 2, even
// though strconv accepts a leading sign.
func TestDeleteAccountRefusesOwnAndBadIDs(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	resp, _ := s.session(admin)
	adminID := resp.User.ID
	member := s.createAccount(admin, "member", "member password", false)
	memberID := strconv.FormatInt(member.ID, 10)

	expectError(t, s.deleteUserAs(admin, adminID), http.StatusBadRequest, "own_account")
	if !s.signedIn(admin) {
		t.Error("deleting your own account ended your session")
	}

	for _, p := range []string{
		"/api/v1/users/99999",
		"/api/v1/users/0",
		"/api/v1/users/-1",
		"/api/v1/users/abc",
		"/api/v1/users/1.5",
		"/api/v1/users/+" + memberID,
		"/api/v1/users/%2B" + memberID,
	} {
		t.Run("DELETE "+p, func(t *testing.T) {
			expectError(t, s.do(request{method: http.MethodDelete, path: p, cookie: admin}), http.StatusNotFound, "not_found")
		})
		t.Run("PUT "+p+"/password", func(t *testing.T) {
			rec := s.do(request{method: http.MethodPut, path: p + "/password", cookie: admin, body: jsonBody(t, map[string]string{"password": "hijacked password"})})
			expectError(t, rec, http.StatusNotFound, "not_found")
		})
	}

	if got := usernames(s.listAccounts(admin)); !slices.Equal(got, []string{"admin", "member"}) {
		t.Errorf("accounts = %v, want [admin member]", got)
	}
	s.login("member", "member password")
}

// Resetting a password ends that account's sessions. Refused resets change
// nothing.
func TestResetPasswordEndsTheMembersSessions(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	resp, _ := s.session(admin)
	adminID := resp.User.ID
	member := s.createAccount(admin, "member", "member password", false)
	first := s.login("member", "member password")
	second := s.login("member", "member password")

	expectError(t, s.resetUserPassword(admin, adminID, "reset password"), http.StatusBadRequest, "own_account")
	expectError(t, s.resetUserPassword(admin, member.ID, "short"), http.StatusBadRequest, "invalid_input")
	expectError(t, s.resetUserPassword(admin, 99999, "reset password"), http.StatusNotFound, "not_found")
	if !s.signedIn(first) || !s.signedIn(second) || !s.signedIn(admin) {
		t.Fatal("a refused reset ended a session")
	}
	s.login("member", "member password")

	if rec := s.resetUserPassword(admin, member.ID, "reset password"); rec.Code != http.StatusNoContent {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body)
	}
	if s.signedIn(first) || s.signedIn(second) {
		t.Error("a session of the member survived the reset")
	}
	s.expectLoginRefused("member", "member password")
	s.login("member", "reset password")
	if !s.signedIn(admin) {
		t.Error("the admin's own session ended")
	}
}

// Changing your password ends your other sessions and keeps this one.
func TestChangePasswordEndsTheOtherSessions(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	s.createAccount(admin, "member", "member password", false)
	first := s.login("member", "member password")
	second := s.login("member", "member password")
	if !s.signedIn(first) || !s.signedIn(second) {
		t.Fatal("the member's sessions don't work before the change")
	}

	if rec := s.changeOwnPassword(first, "member password", "new member password"); rec.Code != http.StatusNoContent {
		t.Fatalf("change: %d %s", rec.Code, rec.Body)
	}
	if !s.signedIn(first) {
		t.Error("the session that changed the password ended")
	}
	if s.signedIn(second) {
		t.Error("another session survived the change")
	}
	s.login("member", "new member password")
	s.expectLoginRefused("member", "member password")
}

// A wrong current password is 400, not 401, and a refused change changes
// nothing.
func TestChangePasswordNeedsTheCurrentPassword(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	s.createAccount(admin, "member", "member password", false)
	first := s.login("member", "member password")
	second := s.login("member", "member password")

	expectError(t, s.changeOwnPassword(first, "not the password", "new member password"), http.StatusBadRequest, "invalid_credentials")
	expectError(t, s.changeOwnPassword(first, "member password", "short"), http.StatusBadRequest, "invalid_input")
	if !s.signedIn(first) || !s.signedIn(second) {
		t.Error("a refused change ended a session")
	}
	s.login("member", "member password")
	s.expectLoginRefused("member", "new member password")
}

// Wrong current passwords use up the account's attempts. The limit is per
// account, so another account's attempts don't count against it.
func TestChangePasswordRateLimitPerAccount(t *testing.T) {
	s := newAuthServer(t, authServerOptions{limiter: auth.NewLimiter(2, time.Hour, nil)})
	admin := s.setup("admin", "admin password")
	s.createAccount(admin, "member", "member password", false)
	member := s.login("member", "member password")

	expectError(t, s.changeOwnPassword(member, "wrong guess", "new member password"), http.StatusBadRequest, "invalid_credentials")
	expectError(t, s.changeOwnPassword(member, "wrong guess", "new member password"), http.StatusBadRequest, "invalid_credentials")
	rec := s.changeOwnPassword(member, "member password", "new member password")
	expectError(t, rec, http.StatusTooManyRequests, "rate_limited")
	if ra := rec.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Errorf("Retry-After = %q, want a positive number of seconds", ra)
	}
	s.login("member", "member password")
	s.expectLoginRefused("member", "new member password")

	expectError(t, s.changeOwnPassword(admin, "wrong guess", "new admin password"), http.StatusBadRequest, "invalid_credentials")
}

// A successful change gives its attempt back, so with one attempt allowed
// two changes in a row both work.
func TestSuccessfulChangeDoesNotUseAnAttempt(t *testing.T) {
	s := newAuthServer(t, authServerOptions{limiter: auth.NewLimiter(1, time.Hour, nil)})
	admin := s.setup("admin", "admin password")
	s.createAccount(admin, "member", "member password", false)
	member := s.login("member", "member password")

	if rec := s.changeOwnPassword(member, "member password", "second member password"); rec.Code != http.StatusNoContent {
		t.Fatalf("first change: %d %s", rec.Code, rec.Body)
	}
	if rec := s.changeOwnPassword(member, "second member password", "third member password"); rec.Code != http.StatusNoContent {
		t.Fatalf("second change: %d %s", rec.Code, rec.Body)
	}
	// One wrong guess uses that single attempt, so the next try waits.
	expectError(t, s.changeOwnPassword(member, "wrong guess", "fourth member password"), http.StatusBadRequest, "invalid_credentials")
	expectError(t, s.changeOwnPassword(member, "third member password", "fourth member password"), http.StatusTooManyRequests, "rate_limited")
}

// Cross-site writes to accounts are refused before they run. Reads aren't
// blocked.
func TestCrossSiteAccountChangesAreBlocked(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	member := s.createAccount(admin, "member", "member password", false)

	for _, req := range []request{
		{method: http.MethodPost, path: "/api/v1/users", body: jsonBody(t, map[string]any{"username": "intruder", "password": "intruder password"})},
		{method: http.MethodDelete, path: idPath(member.ID, "")},
		{method: http.MethodPut, path: idPath(member.ID, "/password"), body: jsonBody(t, map[string]string{"password": "hijacked password"})},
		{method: http.MethodPut, path: "/api/v1/me/password", body: jsonBody(t, map[string]string{"current_password": "admin password", "new_password": "hijacked password"})},
	} {
		req.cookie, req.crossSite = admin, true
		t.Run(req.method+" "+req.path, func(t *testing.T) {
			expectError(t, s.do(req), http.StatusForbidden, "cross_origin")
		})
	}

	if rec := s.do(request{method: http.MethodGet, path: "/api/v1/users", cookie: admin, crossSite: true}); rec.Code != http.StatusOK {
		t.Errorf("cross-site GET: %d, want 200", rec.Code)
	}
	if got := usernames(s.listAccounts(admin)); !slices.Equal(got, []string{"admin", "member"}) {
		t.Errorf("accounts = %v, want [admin member]", got)
	}
	s.login("member", "member password")
	s.login("admin", "admin password")
}

// Passwords and session tokens never appear in a response body or the log.
func TestAccountResponsesAndLogsNeverContainSecrets(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "Sekrit-admin-1")
	var bodies []string
	track := func(rec *httptest.ResponseRecorder) *httptest.ResponseRecorder {
		bodies = append(bodies, rec.Body.String())
		return rec
	}

	rec := track(s.do(request{
		method: http.MethodPost, path: "/api/v1/users", cookie: admin,
		body: jsonBody(t, map[string]any{"username": "member", "password": "Sekrit-member-2"}),
	}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created struct {
		User accountJSON `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	member := s.login("member", "Sekrit-member-2")
	track(s.do(request{method: http.MethodGet, path: "/api/v1/users", cookie: admin}))
	track(s.changeOwnPassword(member, "Sekrit-member-2", "Sekrit-member-3"))
	track(s.changeOwnPassword(member, "Sekrit-guess-5", "Sekrit-member-3"))
	track(s.resetUserPassword(admin, created.User.ID, "Sekrit-reset-4"))
	track(s.do(request{method: http.MethodPost, path: "/api/v1/auth/login", body: creds("member", "Sekrit-member-3")}))
	s.login("member", "Sekrit-reset-4")
	track(s.do(request{
		method: http.MethodPost, path: "/api/v1/users", cookie: admin,
		body: jsonBody(t, map[string]any{"username": "bad name", "password": "Sekrit-guess-5"}),
	}))

	logs := s.logs.String()
	if !strings.Contains(logs, "path=/api/v1/users") {
		t.Fatalf("the log has no request lines to check:\n%s", logs)
	}
	for _, secret := range []string{"Sekrit-admin-1", "Sekrit-member-2", "Sekrit-member-3", "Sekrit-reset-4", "Sekrit-guess-5", admin.Value, member.Value} {
		for _, body := range bodies {
			if strings.Contains(body, secret) {
				t.Errorf("a response body contains %q: %s", secret, body)
			}
		}
		if strings.Contains(logs, secret) {
			t.Errorf("the log contains %q:\n%s", secret, logs)
		}
	}
}

// Account endpoints that hash a password have the same deadline as login:
// 503 when every hashing slot stays busy.
func TestAccountEndpointsBusy(t *testing.T) {
	s := newAuthServer(t, authServerOptions{loginWait: time.Nanosecond})
	admin := s.setup("admin", "long enough")
	for _, req := range []request{
		{method: "POST", path: "/api/v1/users", body: `{"username":"bob","password":"long enough"}`},
		{method: "PUT", path: "/api/v1/me/password", body: `{"current_password":"long enough","new_password":"new password"}`},
	} {
		req.cookie = admin
		rec := s.do(req)
		if rec.Code != http.StatusServiceUnavailable || decodeError(t, rec).Code != "busy" {
			t.Errorf("%s %s: %d %s, want 503 busy", req.method, req.path, rec.Code, rec.Body)
		}
	}
	if strings.Contains(s.logs.String(), "level=ERROR") {
		t.Errorf("a busy server logged errors:\n%s", s.logs)
	}
}

// A rejected new password doesn't judge a guess, so it doesn't use up an
// attempt.
func TestInvalidNewPasswordDoesNotUseAnAttempt(t *testing.T) {
	s := newAuthServer(t, authServerOptions{limiter: auth.NewLimiter(1, time.Hour, nil)})
	admin := s.setup("admin", "long enough")
	for range 3 {
		rec := s.do(request{method: "PUT", path: "/api/v1/me/password", cookie: admin,
			body: `{"current_password":"long enough","new_password":"short"}`})
		if rec.Code != http.StatusBadRequest || decodeError(t, rec).Code != "invalid_input" {
			t.Fatalf("short new password: %d %s, want 400 invalid_input", rec.Code, rec.Body)
		}
	}
	rec := s.do(request{method: "PUT", path: "/api/v1/me/password", cookie: admin,
		body: `{"current_password":"long enough","new_password":"new password"}`})
	if rec.Code != http.StatusNoContent {
		t.Errorf("valid change after rejected ones: %d %s, want 204", rec.Code, rec.Body)
	}
}

// An admin whose account is deleted while their request is in flight (here:
// before it arrives, with the cookie still in hand) gets 401 and the cookie
// cleared, not a 403.
func TestDeletedAdminGets401(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	a := s.setup("admin", "long enough")
	rec := s.do(request{method: "POST", path: "/api/v1/users", cookie: a, body: `{"username":"b","password":"long enough","is_admin":true}`})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create b: %d %s", rec.Code, rec.Body)
	}
	b := sessionCookieFrom(t, s.do(request{method: "POST", path: "/api/v1/auth/login", body: creds("b", "long enough")}))
	if _, err := s.db.Write.Exec("DELETE FROM users WHERE username = 'b'"); err != nil {
		t.Fatal(err)
	}
	rec = s.do(request{method: "POST", path: "/api/v1/users", cookie: b, body: `{"username":"c","password":"long enough"}`})
	if rec.Code != http.StatusUnauthorized || decodeError(t, rec).Code != "unauthenticated" {
		t.Errorf("deleted admin creates an account: %d %s, want 401 unauthenticated", rec.Code, rec.Body)
	}
}

// Behind a reverse proxy with HTTP Basic auth, the browser sends an
// Authorization header with every request; account management must still
// work with the session cookie.
func TestAccountsWorkBehindBasicAuthProxy(t *testing.T) {
	s := newAuthServer(t, authServerOptions{})
	admin := s.setup("admin", "admin password")
	basic := map[string]string{"Authorization": "Basic dXNlcjpwYXNz"}
	if rec := s.do(request{method: http.MethodGet, path: "/api/v1/users", cookie: admin, header: basic}); rec.Code != http.StatusOK {
		t.Errorf("GET /users with a session and Basic auth: %d %s, want 200", rec.Code, rec.Body)
	}
	rec := s.do(request{method: http.MethodPut, path: "/api/v1/me/password", cookie: admin, header: basic,
		body: `{"current_password":"admin password","new_password":"new password"}`})
	if rec.Code != http.StatusNoContent {
		t.Errorf("PUT /me/password with a session and Basic auth: %d %s, want 204", rec.Code, rec.Body)
	}
}
