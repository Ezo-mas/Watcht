package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/db/sqlc"
)

var (
	// ErrInvalidTokenName wraps a message explaining the naming rule.
	ErrInvalidTokenName = errors.New("invalid token name")
	// ErrTokenNameTaken means the user already has a token with this name.
	ErrTokenNameTaken = errors.New("token name is taken")
	// ErrNoToken means an API token is malformed, unknown, or revoked.
	ErrNoToken = errors.New("no valid API token")
)

// apiTokenPrefix marks watcht API tokens, so they're recognizable in
// config files and by secret scanners, and can't be confused with a
// session cookie's value.
const apiTokenPrefix = "wt_"

const maxTokenNameChars = 64

// APIToken describes a token; the secret itself is only returned once, at
// creation.
type APIToken struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

// CreateAPIToken makes a token that acts as the actor on the data API, and
// returns its secret, which is shown once and never stored.
func (s *Service) CreateAPIToken(ctx context.Context, actor Session, name string) (APIToken, string, error) {
	name = strings.TrimSpace(name)
	if err := validateTokenName(name); err != nil {
		return APIToken{}, "", err
	}
	raw, hash := newToken()
	var tok APIToken
	err := s.inTx(ctx, func(q *sqlc.Queries) error {
		if _, err := s.recheckSession(ctx, q, actor); err != nil {
			return err
		}
		row, err := q.CreateAPIToken(ctx, sqlc.CreateAPITokenParams{
			UserID:    actor.User.ID,
			Name:      name,
			TokenHash: hash,
			CreatedAt: db.Timestamp(s.now()),
		})
		if db.IsUniqueViolation(err) {
			// token_hash is random, so the clash is the name.
			return ErrTokenNameTaken
		}
		if err != nil {
			return fmt.Errorf("create API token: %w", err)
		}
		tok, err = toAPIToken(row.ID, row.Name, row.CreatedAt)
		return err
	})
	if err != nil {
		return APIToken{}, "", err
	}
	return tok, apiTokenPrefix + raw, nil
}

// ListAPITokens returns the actor's tokens, oldest first.
func (s *Service) ListAPITokens(ctx context.Context, actor Session) ([]APIToken, error) {
	rows, err := s.read.ListAPITokens(ctx, actor.User.ID)
	if err != nil {
		return nil, fmt.Errorf("list API tokens: %w", err)
	}
	tokens := make([]APIToken, 0, len(rows))
	for _, r := range rows {
		t, err := toAPIToken(r.ID, r.Name, r.CreatedAt)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, nil
}

// RevokeAPIToken deletes one of the actor's tokens; it stops working at
// once. Another user's token is ErrNotFound, like a missing one.
func (s *Service) RevokeAPIToken(ctx context.Context, actor Session, id int64) error {
	n, err := s.write.DeleteAPIToken(ctx, sqlc.DeleteAPITokenParams{ID: id, UserID: actor.User.ID})
	if err != nil {
		return fmt.Errorf("revoke API token %d: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AuthenticateAPIToken returns the user a token acts as.
func (s *Service) AuthenticateAPIToken(ctx context.Context, token string) (User, error) {
	raw, ok := strings.CutPrefix(token, apiTokenPrefix)
	if !ok {
		return User{}, ErrNoToken
	}
	hash, ok := hashToken(raw)
	if !ok {
		return User{}, ErrNoToken
	}
	row, err := s.read.GetAPITokenUser(ctx, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNoToken
	}
	if err != nil {
		return User{}, fmt.Errorf("look up API token: %w", err)
	}
	return User{ID: row.ID, Username: row.Username, IsAdmin: row.IsAdmin == 1}, nil
}

func validateTokenName(name string) error {
	n := utf8.RuneCountInString(name)
	if n == 0 || n > maxTokenNameChars || !utf8.ValidString(name) {
		return fmt.Errorf("%w: use 1-%d characters", ErrInvalidTokenName, maxTokenNameChars)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: no control characters", ErrInvalidTokenName)
		}
	}
	return nil
}

func toAPIToken(id int64, name, createdAt string) (APIToken, error) {
	created, err := db.ParseTimestamp(createdAt)
	if err != nil {
		return APIToken{}, err
	}
	return APIToken{ID: id, Name: name, CreatedAt: created}, nil
}
