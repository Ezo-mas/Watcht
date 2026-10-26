package auth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAPITokenLifecycle(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	admin, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}

	tok, secret, err := s.CreateAPIToken(ctx, admin, "  jellyfin  ")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}
	if tok.Name != "jellyfin" || tok.ID == 0 || tok.CreatedAt.IsZero() {
		t.Errorf("token = %+v; want the trimmed name and an ID", tok)
	}
	if !strings.HasPrefix(secret, "wt_") || len(secret) != 3+43 {
		t.Errorf("secret = %q, want wt_ and 43 base64url characters", secret)
	}

	u, err := s.AuthenticateAPIToken(ctx, secret)
	if err != nil || u != admin.User {
		t.Errorf("AuthenticateAPIToken = %+v, %v; want %+v", u, err, admin.User)
	}

	// Only the hash is stored.
	var stored []byte
	if err := d.Read.QueryRowContext(ctx, "SELECT token_hash FROM api_tokens").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	want, _ := hashToken(strings.TrimPrefix(secret, "wt_"))
	if !bytes.Equal(stored, want) || bytes.Contains(stored, []byte(secret)) {
		t.Errorf("stored token_hash = %x, want the SHA-256 of the token", stored)
	}

	list, err := s.ListAPITokens(ctx, admin)
	if err != nil || len(list) != 1 || list[0] != tok {
		t.Errorf("ListAPITokens = %+v, %v; want [%+v]", list, err, tok)
	}

	if err := s.RevokeAPIToken(ctx, admin, tok.ID); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}
	if _, err := s.AuthenticateAPIToken(ctx, secret); !errors.Is(err, ErrNoToken) {
		t.Errorf("revoked token: %v, want ErrNoToken", err)
	}
	if err := s.RevokeAPIToken(ctx, admin, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoking twice: %v, want ErrNotFound", err)
	}
}

func TestAPITokensBelongToTheirUser(t *testing.T) {
	s, admin, _ := setupWithMember(t)
	ctx := context.Background()
	member, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}
	adminTok, _, err := s.CreateAPIToken(ctx, admin, "script")
	if err != nil {
		t.Fatal(err)
	}
	// Same name, different user: fine.
	_, memberSecret, err := s.CreateAPIToken(ctx, member, "script")
	if err != nil {
		t.Fatalf("same name for another user: %v", err)
	}
	if u, err := s.AuthenticateAPIToken(ctx, memberSecret); err != nil || u.ID != member.User.ID || u.IsAdmin {
		t.Errorf("member's token acts as %+v, %v; want the member", u, err)
	}
	// A member can't see or revoke the admin's token.
	if list, _ := s.ListAPITokens(ctx, member); len(list) != 1 || list[0].ID == adminTok.ID {
		t.Errorf("member's list = %+v; want only their own token", list)
	}
	if err := s.RevokeAPIToken(ctx, member, adminTok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("member revokes admin's token: %v, want ErrNotFound", err)
	}
	if list, _ := s.ListAPITokens(ctx, admin); len(list) != 1 {
		t.Errorf("admin's token was revoked by the member: %+v", list)
	}
}

func TestAPITokenNames(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	admin, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CreateAPIToken(ctx, admin, "Plex – living room 📺"); err != nil {
		t.Errorf("Unicode name: %v", err)
	}
	if _, _, err := s.CreateAPIToken(ctx, admin, strings.Repeat("ą", 64)); err != nil {
		t.Errorf("64 non-ASCII characters: %v", err)
	}
	for name, input := range map[string]string{
		"empty":         "",
		"only spaces":   "   ",
		"65 characters": strings.Repeat("a", 65),
		"newline":       "a\nb",
		"NUL":           "a\x00b",
		"invalid UTF-8": "a\xffb",
	} {
		if _, _, err := s.CreateAPIToken(ctx, admin, input); !errors.Is(err, ErrInvalidTokenName) {
			t.Errorf("%s: %v, want ErrInvalidTokenName", name, err)
		}
	}
	if _, _, err := s.CreateAPIToken(ctx, admin, "dup"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CreateAPIToken(ctx, admin, "dup"); !errors.Is(err, ErrTokenNameTaken) {
		t.Errorf("duplicate name: %v, want ErrTokenNameTaken", err)
	}
}

func TestAuthenticateAPITokenRejects(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	admin, err := s.Setup(ctx, "admin", "long enough")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := s.CreateAPIToken(ctx, admin, "script")
	if err != nil {
		t.Fatal(err)
	}
	unknown, _ := newToken()
	for name, tok := range map[string]string{
		"empty":                  "",
		"no prefix":              strings.TrimPrefix(secret, "wt_"),
		"wrong prefix":           "xx_" + strings.TrimPrefix(secret, "wt_"),
		"unknown":                "wt_" + unknown,
		"truncated":              secret[:len(secret)-1],
		"the session token":      admin.Token,
		"session token prefixed": "wt_" + admin.Token,
	} {
		if _, err := s.AuthenticateAPIToken(ctx, tok); !errors.Is(err, ErrNoToken) {
			t.Errorf("%s: %v, want ErrNoToken", name, err)
		}
	}
}

func TestAPITokensEndWithTheirUser(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()
	m, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := s.CreateAPIToken(ctx, m, "script")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAccount(ctx, admin, member.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AuthenticateAPIToken(ctx, secret); !errors.Is(err, ErrNoToken) {
		t.Errorf("token of a deleted account: %v, want ErrNoToken", err)
	}
}

// Creating a token re-checks the session in the write transaction, so a
// session ended meanwhile (a password reset) can't mint a new credential.
func TestCreateAPITokenNeedsALiveSession(t *testing.T) {
	s, admin, member := setupWithMember(t)
	ctx := context.Background()
	m, err := s.Login(ctx, "member", "member password")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ResetPassword(ctx, admin, member.ID, "reset password"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.CreateAPIToken(ctx, m, "after reset"); !errors.Is(err, ErrNoSession) {
		t.Errorf("CreateAPIToken with an ended session: %v, want ErrNoSession", err)
	}
}
