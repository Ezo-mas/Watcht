package auth

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewToken(t *testing.T) {
	tok, hash := newToken()
	if len(tok) != 43 {
		t.Errorf("token length = %d, want 43 (32 bytes, base64url without padding)", len(tok))
	}
	if len(hash) != 32 {
		t.Errorf("hash length = %d, want 32", len(hash))
	}
	got, ok := hashToken(tok)
	if !ok || !bytes.Equal(got, hash) {
		t.Errorf("hashToken(newToken()) = %x, %v; want the returned hash %x", got, ok, hash)
	}
	if other, _ := newToken(); other == tok {
		t.Error("two tokens are equal")
	}
}

func TestHashTokenRejectsMalformed(t *testing.T) {
	tok, _ := newToken()
	tests := map[string]string{
		"empty":           "",
		"too short":       tok[:42],
		"too long":        tok + "A",
		"padding":         tok[:42] + "=",
		"standard base64": strings.Repeat("+", 43),
		"space":           " " + tok[1:],
		// 43 characters carry 258 bits; the last character's 2 spare bits
		// must be zero, which 'B' violates.
		"non-canonical":    tok[:42] + "B",
		"non-ASCII":        "ą" + tok[2:],
		"null byte inside": tok[:20] + "\x00" + tok[21:],
	}
	for name, s := range tests {
		if _, ok := hashToken(s); ok {
			t.Errorf("%s: hashToken(%q) accepted a malformed token", name, s)
		}
	}
}
