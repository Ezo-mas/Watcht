package db_test

import (
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
