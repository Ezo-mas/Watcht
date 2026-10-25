package auth

import (
	"fmt"
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func newClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 24, 18, 30, 0, 0, time.UTC)}
}

func TestLimiterBurstAndRefill(t *testing.T) {
	clock := newClock()
	l := NewLimiter(3, 10*time.Second, clock.Now)

	for i := range 3 {
		if ok, _ := l.Take("ip"); !ok {
			t.Fatalf("attempt %d refused within the burst", i+1)
		}
	}
	ok, retry := l.Take("ip")
	if ok {
		t.Fatal("4th attempt allowed, want refused")
	}
	if retry != 10*time.Second {
		t.Errorf("retryAfter = %v, want 10s", retry)
	}

	// A refused attempt doesn't use anything up: after 4s, 6s remain.
	clock.Advance(4 * time.Second)
	if ok, retry := l.Take("ip"); ok || retry != 6*time.Second {
		t.Errorf("after 4s: Take = %v, %v; want false, 6s", ok, retry)
	}
	// Partial seconds round up, so a client that waits retryAfter succeeds.
	clock.Advance(5500 * time.Millisecond)
	if ok, retry := l.Take("ip"); ok || retry != time.Second {
		t.Errorf("after 9.5s: Take = %v, %v; want false, 1s", ok, retry)
	}
	clock.Advance(time.Second)
	if ok, _ := l.Take("ip"); !ok {
		t.Error("after 10.5s: refused, want one attempt back")
	}

	// A long wait refills only up to the burst.
	clock.Advance(time.Hour)
	for i := range 3 {
		if ok, _ := l.Take("ip"); !ok {
			t.Fatalf("after an hour: attempt %d refused", i+1)
		}
	}
	if ok, _ := l.Take("ip"); ok {
		t.Error("after an hour: 4th attempt allowed; the bucket must cap at the burst")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	l := NewLimiter(1, time.Minute, newClock().Now)
	if ok, _ := l.Take("a"); !ok {
		t.Fatal("a refused")
	}
	if ok, _ := l.Take("a"); ok {
		t.Fatal("a allowed twice")
	}
	if ok, _ := l.Take("b"); !ok {
		t.Error("b refused because of a")
	}
}

func TestLimiterRefund(t *testing.T) {
	l := NewLimiter(2, time.Minute, newClock().Now)
	l.Take("ip")
	l.Take("ip")
	l.Refund("ip")
	if ok, _ := l.Take("ip"); !ok {
		t.Fatal("refunded attempt not available")
	}
	if ok, _ := l.Take("ip"); ok {
		t.Fatal("more attempts than the burst after a refund")
	}
	// Refunds never raise a bucket above the burst.
	l.Refund("new")
	l.Refund("new")
	l.Refund("new")
	for i := range 2 {
		if ok, _ := l.Take("new"); !ok {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	if ok, _ := l.Take("new"); ok {
		t.Error("refunds raised a bucket above the burst")
	}
}

func TestLimiterPrunesFullBuckets(t *testing.T) {
	clock := newClock()
	l := NewLimiter(2, time.Second, clock.Now)
	for i := range pruneAt {
		l.Take(fmt.Sprint("ip", i))
	}
	l.Take("held")
	l.Take("held")
	// The first pruneAt keys are full again after a second; "held" isn't.
	clock.Advance(time.Second)
	l.Take("trigger")
	if n := len(l.buckets); n != 2 {
		t.Errorf("after pruning, %d buckets tracked; want 2 (held, trigger)", n)
	}
	if ok, _ := l.Take("held"); !ok {
		t.Error("held: refused; it has one attempt back")
	}
	if ok, _ := l.Take("held"); ok {
		t.Error("held: pruning reset a bucket that wasn't full")
	}
}
