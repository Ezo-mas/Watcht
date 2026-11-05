// Package tracking records watch history. It is the only package that
// writes to watch_events, and it owns the rules for doing so:
//
//  1. Source idempotency: an event a source already reported (same user,
//     source, and source event ID) is returned, not stored again.
//  2. Dedupe: a watch of the same item by the same user within the dedupe
//     window of a kept watch is the same watch. It is stored merged into
//     that one, so nothing is lost, and counts once.
//
// The completion rule (85% or marked played) arrives with the media server
// webhooks in M7; every event recorded today is already a completed watch.
package tracking

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

// DefaultDedupeWindow is how close in time two watches of one item must be
// to count as one.
const DefaultDedupeWindow = 3 * time.Hour

// futureSlack is how far past the server's clock a watch time may be, so a
// media server whose clock runs slightly ahead isn't refused.
const futureSlack = 5 * time.Minute

var (
	// ErrInvalid means the event can't be recorded as given: no item or two,
	// no time or one in the future, an unknown source, or a source event ID
	// that doesn't match the source.
	ErrInvalid = errors.New("invalid watch event")
	// ErrNotFound means the movie, episode, or watch isn't stored, or the
	// watch belongs to another user.
	ErrNotFound = errors.New("not found")
)

// Source is where a watch event came from.
type Source string

// The sources watch_events accepts. Every source except SourceManual must
// send a source event ID.
const (
	SourceManual           Source = "manual"
	SourceJellyfin         Source = "jellyfin"
	SourcePlex             Source = "plex"
	SourceTraktImport      Source = "trakt_import"
	SourceLetterboxdImport Source = "letterboxd_import"
)

func (s Source) valid() bool {
	switch s {
	case SourceManual, SourceJellyfin, SourcePlex, SourceTraktImport, SourceLetterboxdImport:
		return true
	}
	return false
}

// Event is one reported watch. Exactly one of MovieID and EpisodeID is set;
// both are internal catalog IDs.
type Event struct {
	UserID    int64
	MovieID   int64
	EpisodeID int64
	WatchedAt time.Time
	// DateOnly means the source gave a calendar date, not a time: the date
	// of WatchedAt in UTC is the watch. It's stored as watched_on, with
	// noon UTC on it as the watch time, so it sorts and dedupes with timed
	// watches.
	DateOnly bool
	Source   Source
	// SourceEventID is the source's own ID for the event. Empty for manual
	// marks, required for every other source.
	SourceEventID string
	// Payload is the event as the source sent it. Optional; must be valid
	// JSON when set.
	Payload json.RawMessage
}

// Result says what Record did with an event.
type Result struct {
	// ID is the stored event's ID; for a duplicate, the earlier event's.
	ID int64
	// WatchedAt is the stored event's watch time.
	WatchedAt time.Time
	// WatchedOn is the stored event's date (YYYY-MM-DD) when it has no time
	// (see Event.DateOnly), or empty.
	WatchedOn string
	// MergedInto is the kept watch this event counts as, or 0 if the event
	// is itself a kept watch.
	MergedInto int64
	// Duplicate is true when the source had already reported this event, so
	// nothing was stored.
	Duplicate bool
}

// Options configures a Service.
type Options struct {
	// DedupeWindow is how close two watches of one item must be to count
	// once. 0 means DefaultDedupeWindow.
	DedupeWindow time.Duration
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
}

// Service records, removes, and lists watches.
type Service struct {
	db     *db.DB
	read   *sqlc.Queries
	write  *sqlc.Queries
	window time.Duration
	now    func() time.Time
}

// New returns a Service that stores into d.
func New(d *db.DB, opts Options) *Service {
	s := &Service{db: d, read: sqlc.New(d.Read), write: sqlc.New(d.Write), window: opts.DedupeWindow, now: opts.Now}
	if s.window <= 0 {
		s.window = DefaultDedupeWindow
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

// Record stores a watch event, applying source idempotency and dedupe in one
// write transaction. The watch time is stored to the whole second.
func (s *Service) Record(ctx context.Context, e Event) (Result, error) {
	if err := s.validate(e); err != nil {
		return Result{}, err
	}
	var res Result
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		var err error
		res, err = s.record(ctx, q, e)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return res, nil
}

// record is Record's work inside the caller's write transaction, so a batch
// (marking a season, an import) can check and insert atomically. e must
// already be validated.
func (s *Service) record(ctx context.Context, q *sqlc.Queries, e Event) (Result, error) {
	watchedAt, watchedOn := e.watchTime()
	if e.SourceEventID != "" {
		prev, err := q.WatchEventBySource(ctx, sqlc.WatchEventBySourceParams{
			UserID: e.UserID, Source: string(e.Source),
			SourceEventID: sql.NullString{String: e.SourceEventID, Valid: true},
		})
		if err == nil {
			at, err := db.ParseTimestamp(prev.WatchedAt)
			if err != nil {
				return Result{}, err
			}
			return Result{ID: prev.ID, WatchedAt: at, WatchedOn: prev.WatchedOn.String, MergedInto: prev.MergedIntoID.Int64, Duplicate: true}, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return Result{}, fmt.Errorf("find source event: %w", err)
		}
	}

	kept, err := s.nearestKept(ctx, q, e, watchedAt)
	if err != nil {
		return Result{}, err
	}
	id, err := q.InsertWatchEvent(ctx, sqlc.InsertWatchEventParams{
		UserID:        e.UserID,
		MovieID:       nullID(e.MovieID),
		EpisodeID:     nullID(e.EpisodeID),
		WatchedAt:     db.Timestamp(watchedAt),
		WatchedOn:     sql.NullString{String: watchedOn, Valid: watchedOn != ""},
		Source:        string(e.Source),
		SourceEventID: sql.NullString{String: e.SourceEventID, Valid: e.SourceEventID != ""},
		Payload:       sql.NullString{String: string(e.Payload), Valid: len(e.Payload) > 0},
		MergedIntoID:  nullID(kept),
		CreatedAt:     db.Timestamp(s.now()),
	})
	if err != nil {
		return Result{}, fmt.Errorf("insert watch event: %w", err)
	}
	return Result{ID: id, WatchedAt: watchedAt, WatchedOn: watchedOn, MergedInto: kept}, nil
}

// watchTime is the time an event is stored at, to the whole second, and its
// date when it has no time: noon UTC on that date.
func (e Event) watchTime() (at time.Time, on string) {
	at = e.WatchedAt.UTC().Truncate(time.Second)
	if !e.DateOnly {
		return at, ""
	}
	y, m, d := at.Date()
	at = time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
	return at, at.Format(time.DateOnly)
}

// nearestKept checks the item exists and returns the kept watch of it
// closest in time to watchedAt within the dedupe window, or 0 if none.
// Equally close watches go to the one recorded first.
func (s *Service) nearestKept(ctx context.Context, q *sqlc.Queries, e Event, watchedAt time.Time) (int64, error) {
	from := db.Timestamp(watchedAt.Add(-s.window))
	to := db.Timestamp(watchedAt.Add(s.window))
	type candidate struct {
		id        int64
		watchedAt string
	}
	var candidates []candidate
	if e.MovieID != 0 {
		ok, err := q.MovieExists(ctx, e.MovieID)
		if err != nil {
			return 0, fmt.Errorf("check movie: %w", err)
		}
		if !ok {
			return 0, ErrNotFound
		}
		rows, err := q.KeptMovieWatches(ctx, sqlc.KeptMovieWatchesParams{
			UserID: e.UserID, MovieID: nullID(e.MovieID), WindowStart: from, WindowEnd: to,
		})
		if err != nil {
			return 0, fmt.Errorf("find movie watches: %w", err)
		}
		for _, r := range rows {
			candidates = append(candidates, candidate{r.ID, r.WatchedAt})
		}
	} else {
		ok, err := q.EpisodeExists(ctx, e.EpisodeID)
		if err != nil {
			return 0, fmt.Errorf("check episode: %w", err)
		}
		if !ok {
			return 0, ErrNotFound
		}
		rows, err := q.KeptEpisodeWatches(ctx, sqlc.KeptEpisodeWatchesParams{
			UserID: e.UserID, EpisodeID: nullID(e.EpisodeID), WindowStart: from, WindowEnd: to,
		})
		if err != nil {
			return 0, fmt.Errorf("find episode watches: %w", err)
		}
		for _, r := range rows {
			candidates = append(candidates, candidate{r.ID, r.WatchedAt})
		}
	}

	var best int64
	var bestGap time.Duration
	for _, c := range candidates { // oldest recorded first, so ties keep the first
		t, err := db.ParseTimestamp(c.watchedAt)
		if err != nil {
			return 0, err
		}
		gap := t.Sub(watchedAt).Abs()
		if best == 0 || gap < bestGap {
			best, bestGap = c.id, gap
		}
	}
	return best, nil
}

// Remove deletes a user's watch: the kept event that eventID is or was
// merged into, and every event merged into it. ErrNotFound if the event
// doesn't exist or isn't the user's.
func (s *Service) Remove(ctx context.Context, userID, eventID int64) error {
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		kept, err := q.KeptWatchID(ctx, sqlc.KeptWatchIDParams{ID: eventID, UserID: userID})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find watch: %w", err)
		}
		n, err := q.DeleteWatch(ctx, sqlc.DeleteWatchParams{ID: kept, UserID: userID})
		if err != nil {
			return fmt.Errorf("delete watch: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("delete watch: kept event %d not deleted", kept)
		}
		return nil
	})
}

func (s *Service) validate(e Event) error {
	at, _ := e.watchTime()
	switch {
	case e.UserID <= 0:
		return fmt.Errorf("%w: no user", ErrInvalid)
	case (e.MovieID == 0) == (e.EpisodeID == 0), e.MovieID < 0, e.EpisodeID < 0:
		return fmt.Errorf("%w: exactly one of movie and episode is required", ErrInvalid)
	case e.WatchedAt.Truncate(time.Second).IsZero(): // stored to the second
		return fmt.Errorf("%w: no watch time", ErrInvalid)
	case at.After(s.now().Add(futureSlack)):
		return fmt.Errorf("%w: watch time is in the future", ErrInvalid)
	// Years 1 to 9999 fit the stored layout; films go back to the 1880s.
	case at.Year() < 1 || at.Year() > 9999:
		return fmt.Errorf("%w: watch time out of range", ErrInvalid)
	case !e.Source.valid():
		return fmt.Errorf("%w: unknown source", ErrInvalid)
	case (e.Source == SourceManual) != (e.SourceEventID == ""):
		return fmt.Errorf("%w: a source event ID is required for every source but manual, and only for those", ErrInvalid)
	case len(e.Payload) > 0 && !json.Valid(e.Payload):
		return fmt.Errorf("%w: payload is not valid JSON", ErrInvalid)
	}
	return nil
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

// nullID stores an optional row ID: 0 is none, so it's NULL.
func nullID(id int64) sql.NullInt64 {
	return sql.NullInt64{Int64: id, Valid: id != 0}
}
