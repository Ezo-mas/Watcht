package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ezo-mas/watcht/internal/db/dbtest"
)

// TestRateLimiterWaits walks a bucket of one token a second, room for two,
// through a sequence of requests and clock steps.
func TestRateLimiterWaits(t *testing.T) {
	clock := &fakeClock{t: testNow}
	l := newRateLimiter(1, 2, clock.Now)
	steps := []struct {
		name    string
		advance time.Duration
		want    time.Duration // the wait reserve returns
	}{
		{"burst token 1", 0, 0},
		{"burst token 2", 0, 0},
		{"out of tokens: wait one second", 0, time.Second},
		{"a second repays the debt, and the next waits again", time.Second, time.Second},
		{"five seconds refill to the burst", 5 * time.Second, 0},
		{"the second token of the burst", 0, 0},
		{"out again", 0, time.Second},
		{"a clock stepping back earns nothing", -10 * time.Second, 2 * time.Second},
	}
	for _, s := range steps {
		clock.Advance(s.advance)
		if got := l.reserve(); got != s.want {
			t.Errorf("%s: reserve() = %v, want %v", s.name, got, s.want)
		}
	}
}

// TestRateLimiterRefund: a token whose wait was cut short goes back, but the
// bucket never holds more than its burst.
func TestRateLimiterRefund(t *testing.T) {
	clock := &fakeClock{t: testNow}
	l := newRateLimiter(1, 1, clock.Now)
	if got := l.reserve(); got != 0 {
		t.Fatalf("first reserve() = %v, want 0", got)
	}
	if got := l.reserve(); got != time.Second {
		t.Fatalf("second reserve() = %v, want 1s", got)
	}
	l.refund()
	if got := l.reserve(); got != time.Second {
		t.Errorf("reserve() after a refund = %v, want 1s: the refunded token should be the one waited for", got)
	}

	l.refund()
	l.refund()
	if got := l.reserve(); got != 0 {
		t.Errorf("reserve() after two refunds = %v, want 0", got)
	}
	if got := l.reserve(); got != time.Second {
		t.Errorf("reserve() beyond the burst = %v, want 1s: refunds can't fill past the burst", got)
	}
}

// TestRateLimitBurstThenWait: the burst goes out at once, then requests wait
// for tokens. Each request reserves its own, so waits add up.
func TestRateLimitBurstThenWait(t *testing.T) {
	e := newEnv(t, okScript("x"), func(o *Options) {
		o.RatePerSecond = 1
		o.Burst = 2
	})
	for range 2 {
		if _, err := e.get("/movie/603", nil, 0); err != nil {
			t.Fatalf("GetJSON during the burst: %v", err)
		}
	}
	if w := e.waits(); len(w) != 0 {
		t.Fatalf("waits during the burst = %v, want none", w)
	}
	for range 2 {
		if _, err := e.get("/movie/603", nil, 0); err != nil {
			t.Fatalf("GetJSON after the burst: %v", err)
		}
	}
	if w := e.waits(); len(w) != 2 || w[0] != time.Second || w[1] != 2*time.Second {
		t.Errorf("waits after the burst = %v, want [1s 2s]", w)
	}
}

// TestRateLimitCancelWhileWaiting: a caller that gives up while waiting for a
// token sends no request and gives the token back.
func TestRateLimitCancelWhileWaiting(t *testing.T) {
	s := okScript("x")
	e := newEnv(t, s, func(o *Options) {
		o.RatePerSecond = 1
		o.Burst = 1
	})
	if _, err := e.get("/movie/603", nil, 0); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.client.sleep = func(wait context.Context, _ time.Duration) error {
		cancel()
		return wait.Err()
	}
	var out payload
	err := e.client.GetJSON(ctx, "/movie/603", nil, 0, &out)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetJSON while waiting for a token = %v, want context.Canceled", err)
	}
	if n := s.count(); n != 1 {
		t.Errorf("requests = %d, want 1: the cancelled call must not send", n)
	}

	// The cancelled wait gave its token back, so the next caller waits one
	// second, not two.
	e.client.sleep = e.recordSleep
	if _, err := e.get("/movie/603", nil, 0); err != nil {
		t.Fatalf("GetJSON after the cancel: %v", err)
	}
	if w := e.waits(); len(w) != 1 || w[0] != time.Second {
		t.Errorf("waits = %v, want [1s] after the refund", w)
	}
}

// TestRateLimiterIgnoresFixedClock: a caller's frozen Now (as in an adapter's
// tests) mustn't freeze the bucket, or every request past the burst would
// wait forever.
func TestRateLimiterIgnoresFixedClock(t *testing.T) {
	d := dbtest.New(t)
	c := New(d, Options{
		Provider:      "test",
		BaseURL:       "http://provider.test",
		RatePerSecond: 100,
		Burst:         1,
		Now:           func() time.Time { return testNow },
	})
	c.limiter.reserve()
	time.Sleep(20 * time.Millisecond) // two tokens' worth at 100/s
	if w := c.limiter.reserve(); w != 0 {
		t.Errorf("wait after the bucket refilled in real time = %v, want 0", w)
	}
}
