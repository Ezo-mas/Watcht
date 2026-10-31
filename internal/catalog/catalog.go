// Package catalog stores TMDB metadata for movies, shows, seasons, and
// episodes, and reads it back. The catalog is shared by every user.
//
// Writes fetch from TMDB first and store afterwards, each in one short write
// transaction. No transaction is open during a TMDB call, because the write
// pool has one connection. Episodes are identified by their TMDB episode ID;
// see the episode identity rule in docs/agent-guides/architecture.md.
package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
	"github.com/Ezo-mas/watcht/internal/jobs"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

var (
	// ErrNotFound means the title isn't stored, or the season or episode is
	// stored but hidden, because TMDB no longer lists it.
	ErrNotFound = errors.New("not found")
	// ErrNotStored means a season was stored for a show that isn't stored.
	// The show is stored first.
	ErrNotStored = errors.New("show is not stored")
)

// Metadata is what catalog needs from TMDB. *tmdb.Client satisfies it.
type Metadata interface {
	Movie(ctx context.Context, id int) (tmdb.Movie, error)
	Show(ctx context.Context, id int) (tmdb.Show, error)
	Season(ctx context.Context, showID, seasonNumber int) (tmdb.Season, error)
}

// Enqueuer is the part of *jobs.Queue that catalog uses.
type Enqueuer interface {
	Enqueue(context.Context, string, any, jobs.EnqueueOptions) error
}

// Options configures a Service.
type Options struct {
	// Jobs enqueues episode syncs after FetchShow. Nil means no enqueue.
	Jobs Enqueuer
	// Logger receives catalog warnings and refresh counts. Nil discards logs.
	Logger *slog.Logger
	// Now is the clock for catalog timestamps. Nil means
	// time.Now.
	Now func() time.Time
}

// Service stores and reads the catalog.
type Service struct {
	db     *db.DB
	read   *sqlc.Queries
	write  *sqlc.Queries
	md     Metadata
	now    func() time.Time
	jobs   Enqueuer
	logger *slog.Logger
}

// New returns a Service that stores into d and fetches from md.
func New(d *db.DB, md Metadata, opts Options) *Service {
	s := &Service{
		db:     d,
		read:   sqlc.New(d.Read),
		write:  sqlc.New(d.Write),
		md:     md,
		now:    opts.Now,
		jobs:   opts.Jobs,
		logger: opts.Logger,
	}
	if s.logger == nil {
		s.logger = slog.New(slog.DiscardHandler)
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

// inTx runs fn in a write transaction (BEGIN IMMEDIATE on the single write
// connection), committing if it returns nil.
func (s *Service) inTx(ctx context.Context, fn func(*sqlc.Queries) error) error {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(s.write.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// nullString stores an optional text value: "" is unknown, so it's NULL.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// nullInt stores an optional number: 0 is unknown, so it's NULL.
func nullInt(n int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(n), Valid: n != 0}
}

// boolInt stores a flag as the 0 or 1 that its CHECK allows.
func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// idList encodes TMDB IDs as the JSON array that the NOT IN checks read. It
// is never "null", which would match no rows at all.
func idList(ids []int64) (string, error) {
	if ids == nil {
		ids = []int64{}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "", fmt.Errorf("encode IDs: %w", err)
	}
	return string(b), nil
}
