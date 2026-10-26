package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// cheap keeps tests fast; TestDefaultParams covers the real cost settings.
var cheap = Argon2Params{Memory: 64, Time: 1, Threads: 1}

func TestHashAndVerify(t *testing.T) {
	ctx := context.Background()
	h := newHasher(cheap, 1)

	a, err := h.hash(ctx, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Errorf("hash = %q, want PHC argon2id format with the configured parameters", a)
	}
	if strings.Contains(a, "correct horse") {
		t.Errorf("hash contains the password: %q", a)
	}
	b, err := h.hash(ctx, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two hashes of the same password are equal; salts must be random")
	}

	for _, tc := range []struct {
		password string
		want     bool
	}{
		{"correct horse", true},
		{"correct horse ", false},
		{"Correct horse", false},
		{"", false},
	} {
		got, err := h.verify(ctx, a, tc.password)
		if err != nil || got != tc.want {
			t.Errorf("verify(%q) = %v, %v; want %v", tc.password, got, err, tc.want)
		}
	}
}

func TestVerifyUsesStoredParams(t *testing.T) {
	ctx := context.Background()
	old, err := newHasher(Argon2Params{Memory: 128, Time: 2, Threads: 2}, 1).hash(ctx, "pw-from-before")
	if err != nil {
		t.Fatal(err)
	}
	// A hasher with different settings still verifies older hashes.
	ok, err := newHasher(cheap, 1).verify(ctx, old, "pw-from-before")
	if err != nil || !ok {
		t.Errorf("verify with changed params = %v, %v; want true", ok, err)
	}
}

func TestDefaultParams(t *testing.T) {
	ctx := context.Background()
	h := newHasher(defaultArgon2Params, 1)
	enc, err := h.hash(ctx, "a real-cost password")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Errorf("hash = %q, want the RFC 9106 parameters", enc)
	}
	if ok, err := h.verify(ctx, enc, "a real-cost password"); err != nil || !ok {
		t.Errorf("verify = %v, %v; want true", ok, err)
	}
}

func TestDummyHash(t *testing.T) {
	ctx := context.Background()
	h := newHasher(cheap, 1)
	d := h.dummyHash()
	p, _, _, err := decodeHash(d)
	if err != nil {
		t.Fatalf("dummy hash doesn't parse: %v", err)
	}
	if p != cheap {
		t.Errorf("dummy hash params = %+v, want %+v (same cost as real hashes)", p, cheap)
	}
	for _, pw := range []string{"", "password", "watcht-dummy-password"} {
		if ok, err := h.verify(ctx, d, pw); err != nil || ok {
			t.Errorf("verify(dummy, %q) = %v, %v; want false, nil", pw, ok, err)
		}
	}
}

func TestDecodeHashRejects(t *testing.T) {
	valid, err := newHasher(cheap, 1).hash(context.Background(), "pw")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(valid, "$")
	salt, key := parts[4], parts[5]
	with := func(params string) string {
		return "$argon2id$v=19$" + params + "$" + salt + "$" + key
	}
	tests := map[string]string{
		"empty":            "",
		"argon2i":          "$argon2i$v=19$m=64,t=1,p=1$" + salt + "$" + key,
		"bcrypt":           "$2a$10$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz01234",
		"old version":      "$argon2id$v=16$m=64,t=1,p=1$" + salt + "$" + key,
		"missing part":     "$argon2id$v=19$m=64,t=1,p=1$" + salt,
		"extra part":       valid + "$x",
		"trailing params":  with("m=64,t=1,p=1x"),
		"reordered params": with("t=1,m=64,p=1"),
		"zero time":        with("m=64,t=0,p=1"),
		"huge time":        with("m=64,t=1000,p=1"),
		"zero threads":     with("m=64,t=1,p=0"),
		"memory too small": with("m=8,t=1,p=2"),
		"huge memory":      with("m=4194304,t=1,p=1"),
		"memory above cap": with("m=262145,t=1,p=1"),
		"time above cap":   with("m=64,t=11,p=1"),
		"bad salt":         "$argon2id$v=19$m=64,t=1,p=1$!!$" + key,
		"short key":        "$argon2id$v=19$m=64,t=1,p=1$" + salt + "$AAAA",
	}
	for name, enc := range tests {
		if _, _, _, err := decodeHash(enc); !errors.Is(err, errBadHash) {
			t.Errorf("%s: decodeHash(%q) error = %v, want errBadHash", name, enc, err)
		}
	}
	if _, _, _, err := decodeHash(valid); err != nil {
		t.Errorf("decodeHash(valid) = %v", err)
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		ok       bool
	}{
		{"8 characters", "abcdefgh", true},
		{"7 characters", "abcdefg", false},
		{"empty", "", false},
		// Counted in characters, not bytes: 7 two-byte letters are too short.
		{"7 non-ASCII characters", "ąčęėįšų", false},
		{"8 non-ASCII characters", "ąčęėįšųū", true},
		{"256 bytes", strings.Repeat("a", 256), true},
		{"257 bytes", strings.Repeat("a", 257), false},
		{"spaces count", "        ", true},
	}
	for _, tc := range tests {
		err := validatePassword(tc.password)
		if tc.ok && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
		if !tc.ok && !errors.Is(err, ErrInvalidPassword) {
			t.Errorf("%s: error = %v, want ErrInvalidPassword", tc.name, err)
		}
	}
}

func TestHashConcurrencyLimit(t *testing.T) {
	h := newHasher(cheap, 1)
	if err := h.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	// With the only slot taken, a hash waits and gives up when its context
	// ends instead of running.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.hash(ctx, "pw"); !errors.Is(err, context.Canceled) {
		t.Errorf("hash with a full semaphore and cancelled context: err = %v, want context.Canceled", err)
	}
	h.release()
	if _, err := h.hash(context.Background(), "pw"); err != nil {
		t.Errorf("hash after release: %v", err)
	}
}
