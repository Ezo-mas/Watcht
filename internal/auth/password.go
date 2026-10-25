package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2Params are the argon2id cost settings. They are stored in each
// hash (PHC string format), so changing them later only affects new hashes.
type Argon2Params struct {
	Memory  uint32 // KiB
	Time    uint32 // passes
	Threads uint8
}

// defaultArgon2Params is RFC 9106's recommendation for memory-constrained
// systems (section 4, second option): 64 MiB, 3 passes, 4 lanes.
var defaultArgon2Params = Argon2Params{Memory: 64 * 1024, Time: 3, Threads: 4}

const (
	saltLen = 16
	keyLen  = 32

	// Password length limits, in bytes for the maximum (it bounds the work
	// done per login attempt) and in characters for the minimum.
	minPasswordChars = 8
	maxPasswordBytes = 256
)

// hasher hashes and verifies passwords. sem bounds how many hashes run at
// once, since each one allocates Params.Memory.
type hasher struct {
	params Argon2Params
	sem    chan struct{}
}

func newHasher(p Argon2Params, concurrency int) *hasher {
	return &hasher{params: p, sem: make(chan struct{}, concurrency)}
}

func (h *hasher) acquire(ctx context.Context) error {
	select {
	case h.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *hasher) release() { <-h.sem }

// hash returns password's argon2id hash in PHC string format:
// $argon2id$v=19$m=65536,t=3,p=4$<salt>$<key>.
func (h *hasher) hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, saltLen)
	_, _ = rand.Read(salt) // crypto/rand.Read never returns an error
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, h.params.Time, h.params.Memory, h.params.Threads, keyLen)
	h.release()
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.params.Memory, h.params.Time, h.params.Threads,
		enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

// dummyHash returns a well-formed hash with h's parameters and an all-zero
// salt and key. Verifying against it costs the same as a real hash and
// never matches (a real key is all zeros with probability 2^-256).
func (h *hasher) dummyHash() string {
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.params.Memory, h.params.Time, h.params.Threads,
		enc.EncodeToString(make([]byte, saltLen)), enc.EncodeToString(make([]byte, keyLen)))
}

// verify reports whether password matches encoded, using the parameters
// stored in encoded.
func (h *hasher) verify(ctx context.Context, encoded, password string) (bool, error) {
	p, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, uint32(len(want))) //nolint:gosec // len(want) is bounded by decodeHash
	h.release()
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

var errBadHash = errors.New("malformed password hash")

// Bounds on parameters read back from a stored hash, so a corrupted or
// tampered row can't make a login allocate gigabytes or run for minutes.
const (
	maxMemory  = 1024 * 1024 // 1 GiB
	maxTime    = 16
	maxStorage = 64 // bytes of salt or key
)

func decodeHash(encoded string) (p Argon2Params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, key
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return p, nil, nil, errBadHash
	}
	if parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return p, nil, nil, errBadHash
	}
	// Sscanf ignores trailing text, so also require the canonical form.
	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads)
	if err != nil || parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", p.Memory, p.Time, p.Threads) {
		return p, nil, nil, errBadHash
	}
	if p.Memory < 8*uint32(p.Threads) || p.Memory > maxMemory || p.Time < 1 || p.Time > maxTime || p.Threads < 1 {
		return p, nil, nil, errBadHash
	}
	enc := base64.RawStdEncoding
	salt, err = enc.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > maxStorage {
		return p, nil, nil, errBadHash
	}
	key, err = enc.DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > maxStorage {
		return p, nil, nil, errBadHash
	}
	return p, salt, key, nil
}

// validatePassword checks the length rules. There are no composition rules
// (NIST SP 800-63B): length is what matters.
func validatePassword(password string) error {
	if len([]rune(password)) < minPasswordChars {
		return fmt.Errorf("%w: use at least %d characters", ErrInvalidPassword, minPasswordChars)
	}
	if len(password) > maxPasswordBytes {
		return fmt.Errorf("%w: use at most %d bytes", ErrInvalidPassword, maxPasswordBytes)
	}
	return nil
}
