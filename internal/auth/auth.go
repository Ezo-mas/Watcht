// Package auth owns accounts and sign-in: password hashing, first-run
// setup, login, and sessions.
//
// Secrets are never stored or logged in the clear: passwords are argon2id
// hashes, and session tokens are stored as SHA-256 hashes. See SECURITY.md
// before changing anything here.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

var (
	// ErrSetupDone means first-run setup was attempted when an account
	// already exists.
	ErrSetupDone = errors.New("setup is already complete")
	// ErrInvalidCredentials means the username or password is wrong. It
	// doesn't say which, so it can't be used to find usernames.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrNoSession means the session token is missing, malformed, unknown,
	// or expired.
	ErrNoSession = errors.New("no valid session")
	// ErrInvalidUsername and ErrInvalidPassword wrap a message explaining
	// the rule that failed.
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidPassword = errors.New("invalid password")
)

const (
	// DefaultSessionTTL is how long a session lasts without use. Each use
	// slides the expiry forward (at most once per sessionRenewEvery).
	DefaultSessionTTL = 30 * 24 * time.Hour
	sessionRenewEvery = 24 * time.Hour

	// hashConcurrency bounds simultaneous password hashes, so concurrent
	// logins can't exhaust memory on a small machine.
	hashConcurrency = 2
)

// Options configures a Service. Zero values mean the defaults.
type Options struct {
	SessionTTL time.Duration
	// Argon2 sets the password hashing cost. Tests use cheap values.
	Argon2 Argon2Params
	// Now is the clock.
	Now func() time.Time
}

// Service implements accounts and sign-in on top of the database.
type Service struct {
	db         *db.DB
	read       *sqlc.Queries
	write      *sqlc.Queries
	hasher     *hasher
	sessionTTL time.Duration
	now        func() time.Time

	// dummyHash is verified against when a login names an unknown user, so
	// a missing account takes as long to reject as a wrong password. It has
	// the current parameters and matches no password.
	dummyHash string
}

// New returns a Service using d.
func New(d *db.DB, opts Options) *Service {
	s := &Service{
		db:         d,
		read:       sqlc.New(d.Read),
		write:      sqlc.New(d.Write),
		hasher:     newHasher(opts.Argon2, hashConcurrency),
		sessionTTL: opts.SessionTTL,
		now:        opts.Now,
	}
	if opts.Argon2 == (Argon2Params{}) {
		s.hasher.params = defaultArgon2Params
	}
	s.dummyHash = s.hasher.dummyHash()
	if s.sessionTTL == 0 {
		s.sessionTTL = DefaultSessionTTL
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

// User is a signed-in account.
type User struct {
	ID       int64
	Username string
	IsAdmin  bool
}

// Session is a sign-in session. Token is the secret for the cookie; it is
// set only when the session is created.
type Session struct {
	User      User
	Token     string
	ExpiresAt time.Time
	// Renewed is true when Authenticate moved the expiry forward, so the
	// caller should refresh the cookie's expiry too.
	Renewed bool
}

// NeedsSetup reports whether no account exists yet.
func (s *Service) NeedsSetup(ctx context.Context) (bool, error) {
	n, err := s.read.CountUsers(ctx)
	if err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return n == 0, nil
}

// Setup creates the first account, an admin, and signs it in. It fails
// with ErrSetupDone once any account exists, including when two setup
// requests race.
func (s *Service) Setup(ctx context.Context, username, password string) (Session, error) {
	if err := validateUsername(username); err != nil {
		return Session{}, err
	}
	if err := validatePassword(password); err != nil {
		return Session{}, err
	}
	// Cheap early exit, so a finished instance doesn't hash for nothing.
	// The check that counts is the one inside the transaction.
	if needs, err := s.NeedsSetup(ctx); err != nil {
		return Session{}, err
	} else if !needs {
		return Session{}, ErrSetupDone
	}
	// Hashing is slow, so it happens before the write transaction starts.
	hash, err := s.hasher.hash(ctx, password)
	if err != nil {
		return Session{}, err
	}

	var sess Session
	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		n, err := q.CountUsers(ctx)
		if err != nil {
			return fmt.Errorf("count users: %w", err)
		}
		if n > 0 {
			return ErrSetupDone
		}
		now := s.now()
		u, err := q.CreateUser(ctx, sqlc.CreateUserParams{
			Username:     username,
			PasswordHash: sql.NullString{String: hash, Valid: true},
			IsAdmin:      1,
			CreatedAt:    db.Timestamp(now),
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		sess, err = s.createSession(ctx, q, User{ID: u.ID, Username: u.Username, IsAdmin: true}, now)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

// Login checks a username and password and starts a session. Any failure
// to match is ErrInvalidCredentials.
func (s *Service) Login(ctx context.Context, username, password string) (Session, error) {
	// Over-long input is rejected before hashing, so it can't buy extra
	// work. Anything else is checked against a hash, even when the user
	// doesn't exist.
	if len(username) > maxUsernameLen || len(password) > maxPasswordBytes {
		return Session{}, ErrInvalidCredentials
	}
	u, err := s.read.GetUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !u.PasswordHash.Valid) {
		if _, err := s.hasher.verify(ctx, s.dummyHash, password); err != nil {
			return Session{}, err
		}
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, fmt.Errorf("look up user: %w", err)
	}
	ok, err := s.hasher.verify(ctx, u.PasswordHash.String, password)
	if err != nil {
		return Session{}, fmt.Errorf("verify password for user %d: %w", u.ID, err)
	}
	if !ok {
		return Session{}, ErrInvalidCredentials
	}

	var sess Session
	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		now := s.now()
		// Expired sessions are cleaned up here, which is often enough for a
		// household; a scheduled job can take over once jobs exist (M3).
		if err := q.DeleteExpiredSessions(ctx, db.Timestamp(now)); err != nil {
			return fmt.Errorf("delete expired sessions: %w", err)
		}
		sess, err = s.createSession(ctx, q, User{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin == 1}, now)
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

func (s *Service) createSession(ctx context.Context, q *sqlc.Queries, u User, now time.Time) (Session, error) {
	token, hash := newToken()
	expires := now.Add(s.sessionTTL)
	err := q.CreateSession(ctx, sqlc.CreateSessionParams{
		TokenHash: hash,
		UserID:    u.ID,
		CreatedAt: db.Timestamp(now),
		ExpiresAt: db.Timestamp(expires),
	})
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return Session{User: u, Token: token, ExpiresAt: expires.UTC().Truncate(time.Second)}, nil
}

// Authenticate returns the session for a token from a cookie, renewing its
// expiry when it's been more than a day since the last renewal. The
// returned Session has no Token.
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	hash, ok := hashToken(token)
	if !ok {
		return Session{}, ErrNoSession
	}
	now := s.now()
	row, err := s.read.GetSession(ctx, sqlc.GetSessionParams{TokenHash: hash, Now: db.Timestamp(now)})
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("look up session: %w", err)
	}
	expires, err := db.ParseTimestamp(row.ExpiresAt)
	if err != nil {
		return Session{}, err
	}
	sess := Session{
		User:      User{ID: row.UserID, Username: row.Username, IsAdmin: row.IsAdmin == 1},
		ExpiresAt: expires,
	}
	// Renew at most once a day, so most requests don't write.
	if newExpiry := now.Add(s.sessionTTL).UTC().Truncate(time.Second); newExpiry.Sub(expires) >= sessionRenewEvery {
		err := s.write.ExtendSession(ctx, sqlc.ExtendSessionParams{ID: row.ID, ExpiresAt: db.Timestamp(newExpiry)})
		if err != nil {
			return Session{}, fmt.Errorf("renew session: %w", err)
		}
		sess.ExpiresAt, sess.Renewed = newExpiry, true
	}
	return sess, nil
}

// Logout ends the session for token. An unknown token is not an error.
func (s *Service) Logout(ctx context.Context, token string) error {
	hash, ok := hashToken(token)
	if !ok {
		return nil
	}
	if err := s.write.DeleteSession(ctx, hash); err != nil {
		return fmt.Errorf("delete session: %w", err)
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

const maxUsernameLen = 64

// usernamePattern matches the schema's rule (migration 00001): 1-64 ASCII
// letters, digits, '.', '_' or '-'. Checking here too gives a clear error
// instead of a constraint failure.
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func validateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("%w: use 1-%d letters, digits, '.', '_' or '-'", ErrInvalidUsername, maxUsernameLen)
	}
	return nil
}
