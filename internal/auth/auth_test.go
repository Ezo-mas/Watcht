package auth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

func newService(t *testing.T) (*Service, *db.DB, *fakeClock) {
	t.Helper()
	d := dbtest.New(t)
	clock := newClock()
	return New(d, Options{Argon2: cheap, Now: clock.Now}), d, clock
}

func TestSetup(t *testing.T) {
	s, d, clock := newService(t)
	ctx := context.Background()

	if needs, err := s.NeedsSetup(ctx); err != nil || !needs {
		t.Fatalf("NeedsSetup on a fresh database = %v, %v; want true", needs, err)
	}
	sess, err := s.Setup(ctx, "Admin", "first password")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if sess.User.Username != "Admin" || !sess.User.IsAdmin || sess.User.ID == 0 {
		t.Errorf("setup user = %+v, want an admin named Admin", sess.User)
	}
	if sess.Token == "" || !sess.ExpiresAt.Equal(clock.Now().Add(DefaultSessionTTL)) {
		t.Errorf("setup session = token %q, expires %v; want a token expiring in 30 days", sess.Token, sess.ExpiresAt)
	}
	if needs, err := s.NeedsSetup(ctx); err != nil || needs {
		t.Errorf("NeedsSetup after setup = %v, %v; want false", needs, err)
	}

	// The password is stored as an argon2id hash, never in the clear.
	u, err := sqlc.New(d.Read).GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u.PasswordHash.String, "$argon2id$") || strings.Contains(u.PasswordHash.String, "first password") {
		t.Errorf("stored password hash = %q", u.PasswordHash.String)
	}

	// The setup session works, and setup can't run again.
	if _, err := s.Authenticate(ctx, sess.Token); err != nil {
		t.Errorf("Authenticate(setup token): %v", err)
	}
	if _, err := s.Setup(ctx, "other", "second password"); !errors.Is(err, ErrSetupDone) {
		t.Errorf("second Setup error = %v, want ErrSetupDone", err)
	}
}

func TestSetupValidates(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	if _, err := s.Setup(ctx, "bad name", "long enough"); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("Setup with a space in the username: err = %v, want ErrInvalidUsername", err)
	}
	if _, err := s.Setup(ctx, strings.Repeat("a", 65), "long enough"); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("Setup with a 65-character username: err = %v, want ErrInvalidUsername", err)
	}
	if _, err := s.Setup(ctx, "admin", "short"); !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("Setup with a short password: err = %v, want ErrInvalidPassword", err)
	}
	if needs, _ := s.NeedsSetup(ctx); !needs {
		t.Error("a rejected Setup created an account")
	}
}

// Concurrent setup requests must create exactly one account: the check
// inside the write transaction decides, not the early exit.
func TestSetupRace(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Go(func() {
			_, errs[i] = s.Setup(ctx, "admin"+string(rune('a'+i)), "long enough")
		})
	}
	wg.Wait()

	var ok int
	for i, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, ErrSetupDone):
			t.Errorf("Setup %d: unexpected error %v", i, err)
		}
	}
	if ok != 1 {
		t.Errorf("%d setups succeeded, want exactly 1", ok)
	}
	if count, err := sqlc.New(d.Read).CountUsers(ctx); err != nil || count != 1 {
		t.Errorf("CountUsers = %d, %v; want 1", count, err)
	}
}

func TestLogin(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	if _, err := s.Setup(ctx, "Alice", "alice password"); err != nil {
		t.Fatal(err)
	}
	// A user without a password (OIDC-only, later) can't sign in with one.
	if _, err := sqlc.New(d.Write).CreateUser(ctx, sqlc.CreateUserParams{Username: "nopass", CreatedAt: db.Timestamp(time.Now())}); err != nil {
		t.Fatal(err)
	}

	sess, err := s.Login(ctx, "alice", "alice password")
	if err != nil {
		t.Fatalf("Login with the right password (username in another case): %v", err)
	}
	if sess.User.Username != "Alice" || !sess.User.IsAdmin || sess.Token == "" {
		t.Errorf("login session = %+v", sess)
	}
	if got, err := s.Authenticate(ctx, sess.Token); err != nil || got.User != sess.User {
		t.Errorf("Authenticate(login token) = %+v, %v; want user %+v", got.User, err, sess.User)
	}

	// Every failure is the same error, so it can't reveal which usernames
	// exist.
	for _, tc := range []struct{ name, username, password string }{
		{"wrong password", "alice", "wrong password"},
		{"unknown user", "bob", "alice password"},
		{"no password set", "nopass", "anything at all"},
		{"empty", "", ""},
		{"over-long username", strings.Repeat("a", 65), "alice password"},
		{"over-long password", "alice", strings.Repeat("a", 257)},
	} {
		if _, err := s.Login(ctx, tc.username, tc.password); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s: Login error = %v, want ErrInvalidCredentials", tc.name, err)
		}
	}
}

func TestSessionTokensAreStoredHashed(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	sess, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	var stored []byte
	if err := d.Read.QueryRowContext(ctx, "SELECT token_hash FROM sessions").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	want, _ := hashToken(sess.Token)
	if !bytes.Equal(stored, want) {
		t.Errorf("stored token_hash = %x, want SHA-256 of the token %x", stored, want)
	}
	if bytes.Contains(stored, []byte(sess.Token)) {
		t.Error("the session token is stored in the clear")
	}
}

func TestSessionExpiryAndRenewal(t *testing.T) {
	s, _, clock := newService(t)
	ctx := context.Background()
	sess, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	created := clock.Now()

	// Within a day of the last renewal, the expiry doesn't move (no write).
	clock.Advance(23 * time.Hour)
	got, err := s.Authenticate(ctx, sess.Token)
	if err != nil || got.Renewed || !got.ExpiresAt.Equal(created.Add(DefaultSessionTTL)) {
		t.Errorf("after 23h: %+v, %v; want unchanged expiry, not renewed", got, err)
	}
	if got.Token != "" {
		t.Error("Authenticate returned the token; only Setup and Login should")
	}

	// After a day, it slides to 30 days from now.
	clock.Advance(2 * time.Hour)
	got, err = s.Authenticate(ctx, sess.Token)
	if err != nil || !got.Renewed || !got.ExpiresAt.Equal(clock.Now().Add(DefaultSessionTTL)) {
		t.Errorf("after 25h: %+v, %v; want renewed to now + 30 days", got, err)
	}

	// The renewal was stored: the session outlives its original expiry
	// (and, being used again, renews again).
	clock.t = created.Add(DefaultSessionTTL).Add(time.Hour)
	got, err = s.Authenticate(ctx, sess.Token)
	if err != nil {
		t.Fatalf("after the original expiry: %v; want the renewed session to still work", err)
	}
	renewed := got.ExpiresAt

	// Unused past its expiry, it's gone.
	clock.t = renewed.Add(time.Hour)
	if _, err := s.Authenticate(ctx, sess.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after expiry: err = %v, want ErrNoSession", err)
	}
	// Exactly at the expiry time, it's already expired.
	clock.t = renewed
	if _, err := s.Authenticate(ctx, sess.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("at the expiry instant: err = %v, want ErrNoSession", err)
	}
}

func TestLoginDeletesExpiredSessions(t *testing.T) {
	s, d, clock := newService(t)
	ctx := context.Background()
	if _, err := s.Setup(ctx, "admin", "long enough"); err != nil {
		t.Fatal(err)
	}
	clock.Advance(DefaultSessionTTL + time.Hour)
	if _, err := s.Login(ctx, "admin", "long enough"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM sessions").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("%d sessions stored, want 1 (the expired setup session deleted)", n)
	}
}

func TestAuthenticateRejects(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	if _, err := s.Setup(ctx, "admin", "long enough"); err != nil {
		t.Fatal(err)
	}
	unknown, _ := newToken()
	for name, tok := range map[string]string{
		"empty":     "",
		"malformed": "not-a-token",
		"unknown":   unknown,
	} {
		if _, err := s.Authenticate(ctx, tok); !errors.Is(err, ErrNoSession) {
			t.Errorf("%s: err = %v, want ErrNoSession", name, err)
		}
	}
}

func TestLogout(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	a, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Login(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(ctx, a.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, a.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after logout: err = %v, want ErrNoSession", err)
	}
	// Only that session ends.
	if _, err := s.Authenticate(ctx, b.Token); err != nil {
		t.Errorf("other session after logout: %v", err)
	}
	// Logging out twice, or with garbage, is fine.
	if err := s.Logout(ctx, a.Token); err != nil {
		t.Errorf("second logout: %v", err)
	}
	if err := s.Logout(ctx, "garbage"); err != nil {
		t.Errorf("logout with a malformed token: %v", err)
	}
}

func TestDeletingAUserEndsTheirSessions(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	sess, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.ExecContext(ctx, "DELETE FROM users WHERE id = ?", sess.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, sess.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after deleting the user: err = %v, want ErrNoSession", err)
	}
	var n int
	if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM sessions").Scan(&n); err != nil || n != 0 {
		t.Errorf("sessions left = %d, %v; want 0 (ON DELETE CASCADE)", n, err)
	}
}
