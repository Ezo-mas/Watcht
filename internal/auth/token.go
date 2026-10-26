package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const tokenBytes = 32

// newToken returns a random token for the client and the SHA-256 hash to
// store. The token is 32 random bytes, base64url-encoded without padding.
func newToken() (token string, hash []byte) {
	raw := make([]byte, tokenBytes)
	_, _ = rand.Read(raw) // crypto/rand.Read never returns an error
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), sum[:]
}

// hashToken returns the stored hash for a token from a client, or false if
// it isn't a well-formed token (so it's rejected without a query).
//
// Tokens are looked up by hash through a unique index rather than compared
// directly: the client controls the token but not its hash, so a lookup's
// timing reveals nothing useful about any stored token.
func hashToken(token string) ([]byte, bool) {
	if len(token) != base64.RawURLEncoding.EncodedLen(tokenBytes) {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	return sum[:], true
}
