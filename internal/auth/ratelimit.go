package auth

import (
	"sync"
	"time"
)

// Limiter is an in-memory token-bucket rate limiter keyed by a string (the
// client IP for logins). Each key starts with Burst tokens and regains one
// every Every. State is lost on restart, which is fine for slowing down
// password guessing.
type Limiter struct {
	burst float64
	every time.Duration
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]bucket
}

type bucket struct {
	tokens float64
	at     time.Time // when tokens was last computed
}

// pruneAt is the number of tracked keys at which full buckets are dropped,
// so a stream of distinct keys can't grow the map without bound.
const pruneAt = 10_000

// NewLimiter returns a limiter allowing burst attempts per key, refilled at
// one per every. now is the clock (nil means time.Now).
func NewLimiter(burst int, every time.Duration, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{burst: float64(burst), every: every, now: now, buckets: map[string]bucket{}}
}

// Take uses one attempt for key. If none is left it returns false and how
// long until one is.
func (l *Limiter) Take(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.buckets) >= pruneAt {
		l.prune(now)
	}
	b := l.refill(key, now)
	if b.tokens < 1 {
		l.buckets[key] = b
		wait := time.Duration((1 - b.tokens) * float64(l.every))
		return false, (wait + time.Second - 1).Truncate(time.Second) // round up
	}
	b.tokens--
	l.buckets[key] = b
	return true, 0
}

// Refund gives back an attempt taken with Take, for an attempt that
// shouldn't count (a successful login).
func (l *Limiter) Refund(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.refill(key, l.now())
	b.tokens = min(l.burst, b.tokens+1)
	l.buckets[key] = b
}

func (l *Limiter) refill(key string, now time.Time) bucket {
	b, ok := l.buckets[key]
	if !ok {
		return bucket{tokens: l.burst, at: now}
	}
	if elapsed := now.Sub(b.at); elapsed > 0 {
		b.tokens = min(l.burst, b.tokens+float64(elapsed)/float64(l.every))
		b.at = now
	}
	return b
}

func (l *Limiter) prune(now time.Time) {
	for k := range l.buckets {
		if l.refill(k, now).tokens >= l.burst {
			delete(l.buckets, k)
		}
	}
}
