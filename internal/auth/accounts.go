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

// ListAccounts returns every account, by username. Admins only.
func (s *Service) ListAccounts(ctx context.Context, actor User) ([]Account, error) {
	if !actor.IsAdmin {
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
func (s *Service) CreateAccount(ctx context.Context, actor User, username, password string, isAdmin bool) (Account, error) {
	if !actor.IsAdmin {
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
	u, err := s.write.CreateUser(ctx, sqlc.CreateUserParams{
		Username:     username,
		PasswordHash: sql.NullString{String: hash, Valid: true},
		IsAdmin:      admin,
		CreatedAt:    db.Timestamp(s.now()),
	})
	if db.IsUniqueViolation(err) {
		return Account{}, ErrUsernameTaken
	}
	if err != nil {
		return Account{}, fmt.Errorf("create user: %w", err)
	}
	return toAccount(u)
}

// DeleteAccount deletes an account with its sessions (and, through foreign
// keys, everything else it owns). Admins only, and not their own account,
// so at least one admin always remains.
func (s *Service) DeleteAccount(ctx context.Context, actor User, id int64) error {
	if !actor.IsAdmin {
		return ErrForbidden
	}
	if id == actor.ID {
		return ErrSelf
	}
	n, err := s.write.DeleteUser(ctx, id)
	if err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ResetPassword sets another account's password, for someone who forgot
// theirs, and signs that account out everywhere. Admins only; an admin
// changes their own password with ChangePassword.
func (s *Service) ResetPassword(ctx context.Context, actor User, id int64, password string) error {
	if !actor.IsAdmin {
		return ErrForbidden
	}
	if id == actor.ID {
		return ErrSelf
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	return s.setPassword(ctx, id, password, 0)
}

// ChangePassword changes the signed-in user's own password after checking
// the current one, and signs out every other session (keepSessionID stays
// signed in). A wrong current password is ErrInvalidCredentials.
func (s *Service) ChangePassword(ctx context.Context, actor User, keepSessionID int64, current, password string) error {
	if len(current) > maxPasswordBytes {
		return ErrInvalidCredentials
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	u, err := s.read.GetUserByID(ctx, actor.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("look up user %d: %w", actor.ID, err)
	}
	// An account without a password (OIDC-only, later) has no current
	// password to give, so it can't set one this way.
	stored := u.PasswordHash.String
	if !u.PasswordHash.Valid {
		stored = s.dummyHash
	}
	ok, err := s.hasher.verify(ctx, stored, current)
	if err != nil {
		return fmt.Errorf("verify password for user %d: %w", actor.ID, err)
	}
	if !ok || !u.PasswordHash.Valid {
		return ErrInvalidCredentials
	}
	return s.setPassword(ctx, actor.ID, password, keepSessionID)
}

// setPassword stores a new hash and ends the account's sessions except
// keepSessionID (0 ends them all), in one transaction, so no session
// signed in with the old password survives the change.
func (s *Service) setPassword(ctx context.Context, id int64, password string, keepSessionID int64) error {
	hash, err := s.hasher.hash(ctx, password)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *sqlc.Queries) error {
		n, err := q.SetPasswordHash(ctx, sqlc.SetPasswordHashParams{ID: id, PasswordHash: sql.NullString{String: hash, Valid: true}})
		if err != nil {
			return fmt.Errorf("set password for user %d: %w", id, err)
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := q.DeleteUserSessions(ctx, sqlc.DeleteUserSessionsParams{UserID: id, KeepID: keepSessionID}); err != nil {
			return fmt.Errorf("end sessions of user %d: %w", id, err)
		}
		return nil
	})
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
