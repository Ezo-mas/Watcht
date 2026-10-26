package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

var (
	// ErrForbidden means the acting user isn't allowed to do this.
	ErrForbidden = errors.New("not allowed")
	// ErrNotFound means the account doesn't exist.
	ErrNotFound = errors.New("no such account")
	// ErrUsernameTaken means another account has this username (ignoring
	// case).
	ErrUsernameTaken = errors.New("username is taken")
	// ErrSelf means an admin action was aimed at the acting admin's own
	// account, which has its own path (deleting yourself isn't allowed;
	// changing your own password needs the current one).
	ErrSelf = errors.New("not allowed on your own account")
)

// Account is a user as shown in account management.
type Account struct {
	User
	CreatedAt time.Time
}

// Account management runs in two steps: slow work (password hashing)
// first, then a write transaction that re-checks everything it relies on.
// Checks made before the transaction can be stale by the time it runs: the
// actor may have been demoted or deleted (and their ID reused), or the
// password changed. The actor is re-checked through their session's token
// hash, which, unlike an ID, is never reused.

// ListAccounts returns every account, by username. Admins only.
func (s *Service) ListAccounts(ctx context.Context, actor Session) ([]Account, error) {
	if !actor.User.IsAdmin {
		return nil, ErrForbidden
	}
	rows, err := s.read.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	accounts := make([]Account, 0, len(rows))
	for _, u := range rows {
		a, err := toAccount(u)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// CreateAccount adds a household account. Admins only; there's no public
// sign-up.
func (s *Service) CreateAccount(ctx context.Context, actor Session, username, password string, isAdmin bool) (Account, error) {
	if !actor.User.IsAdmin {
		return Account{}, ErrForbidden
	}
	if err := validateUsername(username); err != nil {
		return Account{}, err
	}
	if err := validatePassword(password); err != nil {
		return Account{}, err
	}
	hash, err := s.hasher.hash(ctx, password)
	if err != nil {
		return Account{}, err
	}
	var admin int64
	if isAdmin {
		admin = 1
	}
	var created sqlc.User
	err = s.inTx(ctx, func(q *sqlc.Queries) error {
		if _, err := s.recheckAdmin(ctx, q, actor); err != nil {
			return err
		}
		created, err = q.CreateUser(ctx, sqlc.CreateUserParams{
			Username:     username,
			PasswordHash: sql.NullString{String: hash, Valid: true},
			IsAdmin:      admin,
			CreatedAt:    db.Timestamp(s.now()),
		})
		if db.IsUniqueViolation(err) {
			return ErrUsernameTaken
		}
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		return nil
	})
	if err != nil {
		return Account{}, err
	}
	return toAccount(created)
}

// DeleteAccount deletes an account with its sessions (and, through foreign
// keys, everything else it owns). Admins only, and not their own account.
// Because the actor is re-checked in the same transaction, two admins
// deleting each other at once can't both succeed: one admin always
// remains.
func (s *Service) DeleteAccount(ctx context.Context, actor Session, id int64) error {
	if !actor.User.IsAdmin {
		return ErrForbidden
	}
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		admin, err := s.recheckAdmin(ctx, q, actor)
		if err != nil {
			return err
		}
		if id == admin.ID {
			return ErrSelf
		}
		n, err := q.DeleteUser(ctx, id)
		if err != nil {
			return fmt.Errorf("delete user %d: %w", id, err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ResetPassword sets another account's password, for someone who forgot
// theirs, and signs that account out everywhere. Admins only; an admin
// changes their own password with ChangePassword.
func (s *Service) ResetPassword(ctx context.Context, actor Session, id int64, password string) error {
	if !actor.User.IsAdmin {
		return ErrForbidden
	}
	if id == actor.User.ID {
		return ErrSelf
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := s.hasher.hash(ctx, password)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		admin, err := s.recheckAdmin(ctx, q, actor)
		if err != nil {
			return err
		}
		if id == admin.ID {
			return ErrSelf
		}
		n, err := q.SetPasswordHash(ctx, sqlc.SetPasswordHashParams{ID: id, PasswordHash: sql.NullString{String: hash, Valid: true}})
		if err != nil {
			return fmt.Errorf("set password for user %d: %w", id, err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return endSessions(ctx, q, id, 0)
	})
}

// ChangePassword changes the signed-in user's own password after checking
// the current one, and signs out every other session; actor's session
// stays signed in. A wrong current password is ErrInvalidCredentials. If
// the password changes in the meantime (another change, or an admin
// reset), this one fails rather than overwriting it.
func (s *Service) ChangePassword(ctx context.Context, actor Session, current, password string) error {
	if len(current) > maxPasswordBytes {
		return ErrInvalidCredentials
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	u, err := s.read.GetUserByID(ctx, actor.User.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoSession
	}
	if err != nil {
		return fmt.Errorf("look up user %d: %w", actor.User.ID, err)
	}
	// An account without a password (OIDC-only, later) has no current
	// password to give, so it can't set one this way.
	stored := u.PasswordHash.String
	if !u.PasswordHash.Valid {
		stored = s.dummyHash
	}
	ok, err := s.hasher.verify(ctx, stored, current)
	if err != nil {
		return fmt.Errorf("verify password for user %d: %w", actor.User.ID, err)
	}
	if !ok || !u.PasswordHash.Valid {
		return ErrInvalidCredentials
	}
	hash, err := s.hasher.hash(ctx, password)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		// The session must still be live: a reset or a change elsewhere
		// ends it, and so does deleting the account.
		row, err := s.recheckSession(ctx, q, actor)
		if err != nil {
			return err
		}
		n, err := q.ReplacePasswordHash(ctx, sqlc.ReplacePasswordHashParams{
			ID:      actor.User.ID,
			NewHash: sql.NullString{String: hash, Valid: true},
			OldHash: u.PasswordHash,
		})
		if err != nil {
			return fmt.Errorf("set password for user %d: %w", actor.User.ID, err)
		}
		if n == 0 {
			return ErrInvalidCredentials // changed since it was verified
		}
		return endSessions(ctx, q, actor.User.ID, row.ID)
	})
}

// recheckAdmin returns the actor's account as it is now, inside the write
// transaction q belongs to: ErrNoSession if their session has ended (or
// their account was deleted), ErrForbidden if they're no longer an admin.
func (s *Service) recheckAdmin(ctx context.Context, q *sqlc.Queries, actor Session) (User, error) {
	row, err := q.GetSession(ctx, sqlc.GetSessionParams{TokenHash: actor.tokenHash, Now: db.Timestamp(s.now())})
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoSession
	}
	if err != nil {
		return User{}, fmt.Errorf("recheck session: %w", err)
	}
	if row.IsAdmin != 1 {
		return User{}, ErrForbidden
	}
	return User{ID: row.UserID, Username: row.Username, IsAdmin: true}, nil
}

// recheckSession returns the actor's session as it is now, inside the
// write transaction q belongs to, or ErrNoSession if it has ended or no
// longer belongs to the actor's account.
func (s *Service) recheckSession(ctx context.Context, q *sqlc.Queries, actor Session) (sqlc.GetSessionRow, error) {
	row, err := q.GetSession(ctx, sqlc.GetSessionParams{TokenHash: actor.tokenHash, Now: db.Timestamp(s.now())})
	if errors.Is(err, sql.ErrNoRows) || (err == nil && row.UserID != actor.User.ID) {
		return sqlc.GetSessionRow{}, ErrNoSession
	}
	if err != nil {
		return sqlc.GetSessionRow{}, fmt.Errorf("recheck session: %w", err)
	}
	return row, nil
}

// endSessions ends a user's sessions except keep (0 ends them all), so no
// session opened with an old password survives a change.
func endSessions(ctx context.Context, q *sqlc.Queries, userID, keep int64) error {
	if err := q.DeleteUserSessions(ctx, sqlc.DeleteUserSessionsParams{UserID: userID, KeepID: keep}); err != nil {
		return fmt.Errorf("end sessions of user %d: %w", userID, err)
	}
	return nil
}

func toAccount(u sqlc.User) (Account, error) {
	created, err := db.ParseTimestamp(u.CreatedAt)
	if err != nil {
		return Account{}, err
	}
	return Account{
		User:      User{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin == 1},
		CreatedAt: created,
	}, nil
}
