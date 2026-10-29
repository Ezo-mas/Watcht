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

// Expired sessions are deleted by the scheduled job, not by login.
func TestLoginKeepsExpiredSessions(t *testing.T) {
	s, d, clock := newService(t)
	ctx := context.Background()
	if _, err := s.Setup(ctx, "admin", "long enough"); err != nil {
		t.Fatal(err)
	}
	clock.Advance(DefaultSessionTTL + time.Hour)
	if _, err := s.Login(ctx, "admin", "long enough"); err != nil {
		t.Fatal(err)
	}
	if n := countSessions(t, d, ctx); n != 2 {
		t.Errorf("%d sessions stored, want 2 (login must not delete the expired one)", n)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	s, d, clock := newService(t)
	ctx := context.Background()
	if _, err := s.Setup(ctx, "admin", "long enough"); err != nil {
		t.Fatal(err)
	}
	// The setup session expires 30 days after t0; this login session an
	// hour after that.
	clock.Advance(time.Hour)
	if _, err := s.Login(ctx, "admin", "long enough"); err != nil {
		t.Fatal(err)
	}

	if n, err := s.DeleteExpiredSessions(ctx); err != nil || n != 0 {
		t.Fatalf("DeleteExpiredSessions with none expired = %d, %v; want 0", n, err)
	}

	// Past the setup session's expiry but before the login session's.
	clock.Advance(DefaultSessionTTL + 30*time.Minute - time.Hour)
	n, err := s.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("DeleteExpiredSessions = %d, want 1 (only the older session)", n)
	}
	if got := countSessions(t, d, ctx); got != 1 {
		t.Errorf("%d sessions left, want 1", got)
	}

	// At exactly the remaining session's expiry it is expired too.
	clock.Advance(30 * time.Minute)
	n, err = s.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("DeleteExpiredSessions at the expiry instant = %d, want 1", n)
	}
	if got := countSessions(t, d, ctx); got != 0 {
		t.Errorf("%d sessions left after deleting all, want 0", got)
	}
}

func countSessions(t *testing.T, d *db.DB, ctx context.Context) int {
	t.Helper()
	var n int
	if err := d.Read.QueryRowContext(ctx, "SELECT count(*) FROM sessions").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
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

// Every failing login must do the same password-hashing work, or response
// times would show which usernames exist.
func TestLoginAlwaysHashes(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	if _, err := s.Setup(ctx, "alice", "alice password"); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlc.New(d.Write).CreateUser(ctx, sqlc.CreateUserParams{Username: "nopass", CreatedAt: db.Timestamp(time.Now())}); err != nil {
		t.Fatal(err)
	}
	var verifies int
	s.hasher.testHookVerify = func() { verifies++ }

	for _, tc := range []struct {
		name, username, password string
		want                     int
	}{
		{"right password", "alice", "alice password", 1},
		{"wrong password", "alice", "wrong password", 1},
		{"unknown user", "bob", "alice password", 1},
		{"no password set", "nopass", "anything at all", 1},
		// Over-long input is the exception: it's rejected before any work.
		{"over-long username", strings.Repeat("a", 65), "alice password", 0},
		{"over-long password", "alice", strings.Repeat("a", 257), 0},
	} {
		verifies = 0
		_, _ = s.Login(ctx, tc.username, tc.password)
		if verifies != tc.want {
			t.Errorf("%s: %d password verifications, want %d", tc.name, verifies, tc.want)
		}
	}
}

func TestNewRejectsInvalidArgon2Params(t *testing.T) {
	d := dbtest.New(t)
	for _, p := range []Argon2Params{
		{Memory: 64 * 1024}, // partly set: no time or threads
		{Memory: 64, Time: 1, Threads: 0},
		{Memory: 512 * 1024, Time: 1, Threads: 1}, // above the decode bound
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("New with %+v didn't panic", p)
				}
			}()
			New(d, Options{Argon2: p})
		}()
	}
}

func TestSessionRenewalBoundary(t *testing.T) {
	s, _, clock := newService(t)
	ctx := context.Background()
	sess, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(sessionRenewEvery - time.Second)
	if got, err := s.Authenticate(ctx, sess.Token); err != nil || got.Renewed {
		t.Errorf("1s before a day: Renewed = %v, %v; want false", got.Renewed, err)
	}
	clock.Advance(time.Second)
	if got, err := s.Authenticate(ctx, sess.Token); err != nil || !got.Renewed {
		t.Errorf("exactly a day: Renewed = %v, %v; want true", got.Renewed, err)
	}
}

// A TTL shorter than two days renews every half TTL, so it still slides.
func TestShortSessionTTLRenews(t *testing.T) {
	d := dbtest.New(t)
	clock := newClock()
	s := New(d, Options{Argon2: cheap, Now: clock.Now, SessionTTL: 2 * time.Hour})
	ctx := context.Background()
	sess, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Hour)
	got, err := s.Authenticate(ctx, sess.Token)
	if err != nil || !got.Renewed || !got.ExpiresAt.Equal(clock.Now().Add(2*time.Hour)) {
		t.Fatalf("after 1h of a 2h TTL: %+v, %v; want renewed to now + 2h", got, err)
	}
	clock.Advance(90 * time.Minute) // past the original expiry
	if _, err := s.Authenticate(ctx, sess.Token); err != nil {
		t.Errorf("after the original expiry: %v; want the renewed session to work", err)
	}
}

// A renewal that fails to write doesn't sign the user out.
func TestFailedRenewalKeepsSession(t *testing.T) {
	s, d, clock := newService(t)
	ctx := context.Background()
	sess, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.ExecContext(ctx, `CREATE TRIGGER fail_renewal BEFORE UPDATE ON sessions
		BEGIN SELECT RAISE(ABORT, 'renewal blocked by test'); END`); err != nil {
		t.Fatal(err)
	}
	clock.Advance(2 * sessionRenewEvery)
	got, err := s.Authenticate(ctx, sess.Token)
	if err != nil {
		t.Fatalf("Authenticate with a failing renewal: %v; want the session", err)
	}
	if got.Renewed || !got.ExpiresAt.Equal(sess.ExpiresAt) {
		t.Errorf("got Renewed = %v, expires %v; want not renewed, original expiry %v", got.Renewed, got.ExpiresAt, sess.ExpiresAt)
	}
}
