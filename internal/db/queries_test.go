package db_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/Ezo-mas/watcht/internal/db/dbtest"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// These tests cover the schema's constraints through the generated queries,
// which is also the end-to-end check that migrations and sqlc agree.

func TestUsers(t *testing.T) {
	d := dbtest.New(t)
	ctx := context.Background()
	w := sqlc.New(d.Write)
	r := sqlc.New(d.Read)

	n, err := r.CountUsers(ctx)
	if err != nil || n != 0 {
		t.Fatalf("CountUsers on a fresh database = %d, %v; want 0", n, err)
	}

	created, err := w.CreateUser(ctx, sqlc.CreateUserParams{Username: "Alice", CreatedAt: "2026-10-24T18:30:00Z"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.ID == 0 || created.Username != "Alice" {
		t.Errorf("created = %+v", created)
	}

	// Usernames are case-insensitive, for lookup and for uniqueness.
	got, err := r.GetUserByUsername(ctx, "alice")
	if err != nil || got != created {
		t.Errorf("GetUserByUsername(alice) = %+v, %v; want %+v", got, err, created)
	}
	if _, err := w.CreateUser(ctx, sqlc.CreateUserParams{Username: "ALICE", CreatedAt: "2026-10-24T18:31:00Z"}); err == nil {
		t.Error("CreateUser with a username differing only in case succeeded, want a uniqueness error")
	}

	if _, err := r.GetUserByUsername(ctx, "bob"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetUserByUsername(bob) error = %v, want sql.ErrNoRows", err)
	}
	if n, err := r.CountUsers(ctx); err != nil || n != 1 {
		t.Errorf("CountUsers = %d, %v; want 1", n, err)
	}
}

func TestUsernameLength(t *testing.T) {
	d := dbtest.New(t)
	w := sqlc.New(d.Write)
	for _, name := range []string{"", string(make([]byte, 65))} {
		if _, err := w.CreateUser(context.Background(), sqlc.CreateUserParams{Username: name, CreatedAt: "2026-10-24T18:30:00Z"}); err == nil {
			t.Errorf("CreateUser(len %d) succeeded, want a CHECK error", len(name))
		}
	}
}

func TestStrictTablesRejectWrongTypes(t *testing.T) {
	d := dbtest.New(t)
	// STRICT converts values losslessly (12345 into TEXT is fine) but
	// rejects ones that can't be converted, unlike SQLite's default typing.
	_, err := d.Write.Exec("INSERT INTO users (id, username, created_at) VALUES ('not a number', 'carol', '2026-10-24T18:30:00Z')")
	if err == nil {
		t.Fatal("inserting text into an INTEGER column of a STRICT table succeeded, want an error")
	}
}
