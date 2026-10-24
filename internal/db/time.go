package db

import (
	"fmt"
	"time"
)

// TimestampLayout is the one layout for timestamps stored in the database:
// UTC RFC 3339 with whole seconds. A single fixed layout keeps timestamps
// sortable as strings; migrations enforce it with a CHECK.
const TimestampLayout = "2006-01-02T15:04:05Z"

// Timestamp formats t for storage.
func Timestamp(t time.Time) string {
	return t.UTC().Format(TimestampLayout)
}

// ParseTimestamp parses a stored timestamp.
func ParseTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(TimestampLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", s, err)
	}
	return t, nil
}
