package db_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// These tests cover the schema's constraints through the generated queries,
// which is also the end-to-end check that migrations and sqlc agree.

var now = db.Timestamp(time.Date(2026, 10, 24, 18, 30, 0, 0, time.UTC))

var (
	earlier = db.Timestamp(time.Date(2026, 10, 24, 18, 29, 59, 0, time.UTC))
	later   = db.Timestamp(time.Date(2026, 10, 24, 18, 30, 1, 0, time.UTC))
)

// requireConstraint fails unless err is a SQLite constraint error of the
// given kind ("CHECK", "UNIQUE", ...), so a test can't pass for the wrong
// reason.
func requireConstraint(t *testing.T, err error, kind string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got no error, want a %s constraint failure", kind)
	}
	if !strings.Contains(err.Error(), kind+" constraint failed") {
		t.Fatalf("error = %v, want a %s constraint failure", err, kind)
	}
}

// createUser makes a user for sessions to belong to.
func createUser(t *testing.T, w *sqlc.Queries, username string) sqlc.User {
	t.Helper()
	u, err := w.CreateUser(context.Background(), sqlc.CreateUserParams{Username: username, CreatedAt: now})
	if err != nil {
		t.Fatalf("CreateUser(%q): %v", username, err)
	}
	return u
}

// sessionStored reports whether a session with this token exists, whatever
// its expiry. GetSession with a Now before every expiry sees every row.
func sessionStored(t *testing.T, r *sqlc.Queries, token []byte) bool {
	t.Helper()
	_, err := r.GetSession(context.Background(), sqlc.GetSessionParams{TokenHash: token, Now: db.Timestamp(time.Time{})})
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	return true
}

func TestUsers(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	w := sqlc.New(d.Write)
	r := sqlc.New(d.Read)

	n, err := r.CountUsers(ctx)
	if err != nil || n != 0 {
		t.Fatalf("CountUsers on a fresh database = %d, %v; want 0", n, err)
	}

	created, err := w.CreateUser(ctx, sqlc.CreateUserParams{Username: "Alice", CreatedAt: now})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.ID == 0 || created.Username != "Alice" || created.CreatedAt != now {
		t.Errorf("created = %+v", created)
	}

	// Usernames are case-insensitive, for lookup and for uniqueness.
	got, err := r.GetUserByUsername(ctx, "aLiCe")
	if err != nil || got != created {
		t.Errorf("GetUserByUsername(aLiCe) = %+v, %v; want %+v", got, err, created)
	}
	_, err = w.CreateUser(ctx, sqlc.CreateUserParams{Username: "ALICE", CreatedAt: now})
	requireConstraint(t, err, "UNIQUE")

	if _, err := r.GetUserByUsername(ctx, "bob"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetUserByUsername(bob) error = %v, want sql.ErrNoRows", err)
	}
	if n, err := r.CountUsers(ctx); err != nil || n != 1 {
		t.Errorf("CountUsers = %d, %v; want 1", n, err)
	}
}

func TestUsernameRules(t *testing.T) {
	tests := []struct {
		name     string
		username string
		ok       bool
	}{
		{"letters", "alice", true},
		{"digits and punctuation", "a.b_c-1", true},
		{"64 characters", strings.Repeat("a", 64), true},
		{"empty", "", false},
		{"65 characters", strings.Repeat("a", 65), false},
		{"non-ASCII letter", "ärger", false},
		{"decomposed accent", "café", false},
		{"leading space", " alice", false},
		{"inner space", "al ice", false},
		{"only spaces", "   ", false},
		{"NUL byte", "a\x00" + strings.Repeat("c", 10), false},
		{"newline", "alice\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dbtest.New(t)
			_, err := sqlc.New(d.Write).CreateUser(context.Background(), sqlc.CreateUserParams{Username: tt.username, CreatedAt: now})
			if tt.ok {
				if err != nil {
					t.Fatalf("CreateUser(%q) = %v, want success", tt.username, err)
				}
				return
			}
			requireConstraint(t, err, "CHECK")
		})
	}
}

func TestCreatedAtFormat(t *testing.T) {
	tests := []struct {
		value string
		ok    bool
	}{
		{"2026-10-24T18:30:00Z", true},
		{"2026-10-24T18:30:00.5Z", false},
		{"2026-10-24T18:30:00+03:00", false},
		{"2026-10-24 18:30:00", false},
		{"2026-02-30T00:00:00Z", false},
		{"", false},
		{"garbage", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			d := dbtest.New(t)
			_, err := sqlc.New(d.Write).CreateUser(context.Background(), sqlc.CreateUserParams{Username: "alice", CreatedAt: tt.value})
			if tt.ok {
				if err != nil {
					t.Fatalf("CreateUser with created_at %q = %v, want success", tt.value, err)
				}
				return
			}
			requireConstraint(t, err, "CHECK")
		})
	}
}

func TestStrictTablesRejectWrongTypes(t *testing.T) {
	d := dbtest.New(t)
	// A BLOB in a TEXT column is what STRICT rejects and SQLite's default
	// typing would store silently.
	_, err := d.Write.Exec("INSERT INTO users (username, created_at) VALUES (X'616C696365', ?)", now)
	if err == nil || !strings.Contains(err.Error(), "cannot store BLOB value in TEXT column") {
		t.Fatalf("inserting a BLOB into a TEXT column = %v, want a STRICT type error", err)
	}
}

func TestPasswordHashFormat(t *testing.T) {
	valid := func(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }
	tests := []struct {
		name string
		hash sql.NullString
		ok   bool
	}{
		{"NULL", sql.NullString{}, true},
		{"argon2id", valid("$argon2id$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaGhhc2g"), true},
		{"bcrypt", valid("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"), false},
		{"argon2i", valid("$argon2i$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaGhhc2g"), false},
		{"uppercase prefix", valid("$ARGON2ID$v=19$m=65536,t=3,p=4$c2FsdHNhbHQ$aGFzaGhhc2g"), false},
		{"plaintext", valid("plaintext"), false},
		{"empty string", valid(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dbtest.New(t)
			u, err := sqlc.New(d.Write).CreateUser(context.Background(), sqlc.CreateUserParams{Username: "alice", PasswordHash: tt.hash, CreatedAt: now})
			if tt.ok {
				if err != nil {
					t.Fatalf("CreateUser with password_hash %s = %v, want success", tt.name, err)
				}
				if u.PasswordHash != tt.hash {
					t.Errorf("stored password_hash = %+v, want %+v", u.PasswordHash, tt.hash)
				}
				return
			}
			requireConstraint(t, err, "CHECK")
		})
	}
}

func TestIsAdminRange(t *testing.T) {
	tests := []struct {
		name    string
		isAdmin int64
		ok      bool
	}{
		{"member", 0, true},
		{"admin", 1, true},
		{"two", 2, false},
		{"minus one", -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dbtest.New(t)
			u, err := sqlc.New(d.Write).CreateUser(context.Background(), sqlc.CreateUserParams{Username: "alice", IsAdmin: tt.isAdmin, CreatedAt: now})
			if tt.ok {
				if err != nil || u.IsAdmin != tt.isAdmin {
					t.Fatalf("CreateUser with is_admin %d = %+v, %v; want success", tt.isAdmin, u, err)
				}
				return
			}
			requireConstraint(t, err, "CHECK")
		})
	}
}

func TestSessionTokenHashLength(t *testing.T) {
	tests := []struct {
		name   string
		length int
		ok     bool
	}{
		{"32 bytes", 32, true},
		{"31 bytes", 31, false},
		{"33 bytes", 33, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dbtest.New(t)
			ctx := context.Background()
			w := sqlc.New(d.Write)
			user := createUser(t, w, "alice")
			_, err := w.CreateSession(ctx, sqlc.CreateSessionParams{TokenHash: bytes.Repeat([]byte{1}, tt.length), UserID: user.ID, CreatedAt: now, ExpiresAt: later})
			if tt.ok {
				if err != nil {
					t.Fatalf("CreateSession with a %d-byte token_hash = %v, want success", tt.length, err)
				}
				return
			}
			requireConstraint(t, err, "CHECK")
		})
	}
}

func TestSessionTokenHashIsUnique(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	w := sqlc.New(d.Write)
	user := createUser(t, w, "alice")
	session := sqlc.CreateSessionParams{TokenHash: bytes.Repeat([]byte{1}, 32), UserID: user.ID, CreatedAt: now, ExpiresAt: later}
	if _, err := w.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	_, err := w.CreateSession(ctx, session)
	requireConstraint(t, err, "UNIQUE")
}

func TestSessionUserMustExist(t *testing.T) {
	d := dbtest.New(t)
	_, err := sqlc.New(d.Write).CreateSession(context.Background(), sqlc.CreateSessionParams{TokenHash: bytes.Repeat([]byte{1}, 32), UserID: 999, CreatedAt: now, ExpiresAt: later})
	requireConstraint(t, err, "FOREIGN KEY")
}

func TestSessionTimestampFormat(t *testing.T) {
	tests := []struct {
		name      string
		createdAt string
		expiresAt string
		ok        bool
	}{
		{"canonical", now, later, true},
		{"created_at with a space", "2026-10-24 18:30:00", later, false},
		{"created_at with fractional seconds", "2026-10-24T18:30:00.5Z", later, false},
		{"created_at with an offset", "2026-10-24T18:30:00+00:00", later, false},
		{"expires_at with a space", now, "2026-10-24 18:30:00", false},
		{"expires_at with fractional seconds", now, "2026-10-24T18:30:00.5Z", false},
		{"expires_at with an offset", now, "2026-10-24T18:30:00+00:00", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dbtest.New(t)
			w := sqlc.New(d.Write)
			user := createUser(t, w, "alice")
			_, err := w.CreateSession(context.Background(), sqlc.CreateSessionParams{TokenHash: bytes.Repeat([]byte{1}, 32), UserID: user.ID, CreatedAt: tt.createdAt, ExpiresAt: tt.expiresAt})
			if tt.ok {
				if err != nil {
					t.Fatalf("CreateSession = %v, want success", err)
				}
				return
			}
			requireConstraint(t, err, "CHECK")
		})
	}
}

func TestGetSession(t *testing.T) {
	token := bytes.Repeat([]byte{1}, 32)
	tests := []struct {
		name      string
		expiresAt string
		lookup    []byte
		found     bool
	}{
		{"unexpired", later, token, true},
		{"expires exactly now", now, token, false},
		{"expired", earlier, token, false},
		{"unknown token", later, bytes.Repeat([]byte{2}, 32), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dbtest.New(t)
			ctx := context.Background()
			w := sqlc.New(d.Write)
			r := sqlc.New(d.Read)
			root, err := w.CreateUser(ctx, sqlc.CreateUserParams{Username: "root", IsAdmin: 1, CreatedAt: now})
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			if _, err := w.CreateSession(ctx, sqlc.CreateSessionParams{TokenHash: token, UserID: root.ID, CreatedAt: now, ExpiresAt: tt.expiresAt}); err != nil {
				t.Fatalf("CreateSession: %v", err)
			}
			got, err := r.GetSession(ctx, sqlc.GetSessionParams{TokenHash: tt.lookup, Now: now})
			if !tt.found {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("GetSession = %+v, %v; want sql.ErrNoRows", got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetSession: %v", err)
			}
			if got.ExpiresAt != tt.expiresAt || got.UserID != root.ID || got.Username != "root" || got.IsAdmin != 1 {
				t.Errorf("GetSession = %+v, want expires_at %s for user root (admin)", got, tt.expiresAt)
			}
		})
	}
}

func TestExtendSession(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	w := sqlc.New(d.Write)
	r := sqlc.New(d.Read)
	user := createUser(t, w, "alice")
	token := bytes.Repeat([]byte{1}, 32)
	if _, err := w.CreateSession(ctx, sqlc.CreateSessionParams{TokenHash: token, UserID: user.ID, CreatedAt: now, ExpiresAt: later}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := r.GetSession(ctx, sqlc.GetSessionParams{TokenHash: token, Now: now})
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	// The format CHECK applies to UPDATE as well as INSERT.
	requireConstraint(t, w.ExtendSession(ctx, sqlc.ExtendSessionParams{ExpiresAt: "2026-11-24 18:30:00", ID: got.ID}), "CHECK")

	muchLater := db.Timestamp(time.Date(2026, 11, 24, 18, 30, 0, 0, time.UTC))
	if err := w.ExtendSession(ctx, sqlc.ExtendSessionParams{ExpiresAt: muchLater, ID: got.ID}); err != nil {
		t.Fatalf("ExtendSession: %v", err)
	}
	got, err = r.GetSession(ctx, sqlc.GetSessionParams{TokenHash: token, Now: later})
	if err != nil || got.ExpiresAt != muchLater {
		t.Errorf("GetSession after ExtendSession = %+v, %v; want expires_at %s", got, err, muchLater)
	}
}

func TestDeleteSession(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	w := sqlc.New(d.Write)
	r := sqlc.New(d.Read)
	user := createUser(t, w, "alice")
	mine := bytes.Repeat([]byte{1}, 32)
	other := bytes.Repeat([]byte{2}, 32)
	for _, token := range [][]byte{mine, other} {
		if _, err := w.CreateSession(ctx, sqlc.CreateSessionParams{TokenHash: token, UserID: user.ID, CreatedAt: now, ExpiresAt: later}); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	if err := w.DeleteSession(ctx, mine); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if sessionStored(t, r, mine) {
		t.Error("DeleteSession left the session with the matching token_hash")
	}
	if !sessionStored(t, r, other) {
		t.Error("DeleteSession removed a session with a different token_hash")
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	w := sqlc.New(d.Write)
	r := sqlc.New(d.Read)
	user := createUser(t, w, "alice")
	tests := []struct {
		name      string
		expiresAt string
		kept      bool
	}{
		{"expired", earlier, false},
		{"expires exactly now", now, false},
		{"expires later", later, true},
	}
	tokens := make([][]byte, len(tests))
	for i, tt := range tests {
		tokens[i] = bytes.Repeat([]byte{byte(i + 1)}, 32)
		if _, err := w.CreateSession(ctx, sqlc.CreateSessionParams{TokenHash: tokens[i], UserID: user.ID, CreatedAt: now, ExpiresAt: tt.expiresAt}); err != nil {
			t.Fatalf("CreateSession(%s): %v", tt.name, err)
		}
	}
	if err := w.DeleteExpiredSessions(ctx, now); err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	for i, tt := range tests {
		if got := sessionStored(t, r, tokens[i]); got != tt.kept {
			t.Errorf("session %s: stored after DeleteExpiredSessions = %v, want %v", tt.name, got, tt.kept)
		}
	}
}

func TestDeletingUserDeletesSessions(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	w := sqlc.New(d.Write)
	r := sqlc.New(d.Read)
	user := createUser(t, w, "alice")
	token := bytes.Repeat([]byte{1}, 32)
	if _, err := w.CreateSession(ctx, sqlc.CreateSessionParams{TokenHash: token, UserID: user.ID, CreatedAt: now, ExpiresAt: later}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if !sessionStored(t, r, token) {
		t.Fatal("session not stored before its user was deleted")
	}
	// No generated query deletes a user yet, so this is raw SQL.
	if _, err := d.Write.ExecContext(ctx, "DELETE FROM users WHERE id = ?", user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if sessionStored(t, r, token) {
		t.Error("session survived the deletion of its user")
	}
}

func TestIsUniqueViolation(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	w := sqlc.New(d.Write)
	if _, err := w.CreateUser(ctx, sqlc.CreateUserParams{Username: "alice", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	_, err := w.CreateUser(ctx, sqlc.CreateUserParams{Username: "ALICE", CreatedAt: now})
	if !db.IsUniqueViolation(err) {
		t.Errorf("duplicate username: IsUniqueViolation(%v) = false", err)
	}
	_, err = w.CreateUser(ctx, sqlc.CreateUserParams{Username: "bad name", CreatedAt: now})
	if err == nil || db.IsUniqueViolation(err) {
		t.Errorf("CHECK failure: IsUniqueViolation(%v) = true, want false", err)
	}
	if db.IsUniqueViolation(nil) || db.IsUniqueViolation(sql.ErrNoRows) {
		t.Error("IsUniqueViolation is true for a non-constraint error")
	}
}
