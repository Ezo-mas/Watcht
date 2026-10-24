package db

import (
	"sort"
	"testing"
	"time"
)

func TestTimestamp(t *testing.T) {
	vilnius := time.FixedZone("EEST", 3*3600)
	in := time.Date(2026, 10, 24, 21, 30, 15, 999_000_000, vilnius)
	got := Timestamp(in)
	if got != "2026-10-24T18:30:15Z" {
		t.Errorf("Timestamp = %q, want UTC with whole seconds", got)
	}
	back, err := ParseTimestamp(got)
	if err != nil || !back.Equal(in.Truncate(time.Second)) {
		t.Errorf("ParseTimestamp(%q) = %v, %v", got, back, err)
	}
	if _, err := ParseTimestamp("2026-10-24 18:30:15"); err == nil {
		t.Error("ParseTimestamp accepted a non-canonical layout")
	}
}

func TestTimestampsSortAsStrings(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var ts []string
	for _, d := range []time.Duration{3 * time.Hour, 500 * time.Millisecond, 0, 26 * time.Hour, time.Second} {
		ts = append(ts, Timestamp(base.Add(d)))
	}
	sorted := append([]string(nil), ts...)
	sort.Strings(sorted)
	for i := 1; i < len(sorted); i++ {
		a, _ := ParseTimestamp(sorted[i-1])
		b, _ := ParseTimestamp(sorted[i])
		if a.After(b) {
			t.Fatalf("string order %v doesn't match time order", sorted)
		}
	}
}
