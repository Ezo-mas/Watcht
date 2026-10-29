package providers

import (
	"sync"
	"time"
)

// rateLimiter is a token bucket shared by one Client. It starts full with
// burst tokens and gains rate tokens a second, up to burst. reserve takes a
// token even when none is left, so the tokens owed go into debt and each
// caller gets its own wait. Nobody loops on the clock, and a test with a
// fixed clock can't spin.
type rateLimiter struct {
	rate  float64 // tokens per second
	burst float64
	now   func() time.Time

	mu     sync.Mutex
	tokens float64   // negative while callers are waiting for their tokens
	last   time.Time // when tokens was last brought up to date
}

func newRateLimiter(rate float64, burst int, now func() time.Time) *rateLimiter {
	return &rateLimiter{
		rate:   rate,
		burst:  float64(burst),
		now:    now,
		tokens: float64(burst),
		last:   now(),
	}
}

// reserve takes a token and returns how long to wait before using it. The
// wait is zero when a token was available.
func (l *rateLimiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	l.tokens--
	if l.tokens >= 0 {
		return 0
	}
	return time.Duration(-l.tokens / l.rate * float64(time.Second))
}

// refund gives back a token from reserve whose wait was cut short, so it
// isn't lost.
func (l *rateLimiter) refund() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tokens = min(l.tokens+1, l.burst)
}

// refill adds the tokens earned since last. The clock may step back, so last
// only moves forward and a step back earns nothing.
func (l *rateLimiter) refill() {
	now := l.now()
	if !now.After(l.last) {
		return
	}
	l.tokens = min(l.burst, l.tokens+now.Sub(l.last).Seconds()*l.rate)
	l.last = now
}
