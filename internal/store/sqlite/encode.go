package sqlite

import (
	"encoding/json"
	"time"
)

// Column encodings shared by all entities.

// nanos encodes t as unix nanoseconds; the zero time is stored as 0.
func nanos(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// fromNanos is the inverse of nanos and always returns UTC.
func fromNanos(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// encodeStrings returns s as JSON text; nil is "null" and empty is "[]", so
// both survive a round trip.
func encodeStrings(s []string) string {
	b, _ := json.Marshal(s) // a []string cannot fail to marshal
	return string(b)
}

func decodeStrings(text string) ([]string, error) {
	var s []string
	err := json.Unmarshal([]byte(text), &s)
	return s, err
}

// limitArg maps the interface's "limit <= 0 means all" to SQLite's LIMIT -1.
func limitArg(limit int) int {
	if limit <= 0 {
		return -1
	}
	return limit
}
