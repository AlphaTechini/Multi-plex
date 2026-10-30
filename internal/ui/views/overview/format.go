package overview

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// age formats how long ago something happened as a short label: "now",
// "2m", "6h", "3d".
func age(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	default:
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	}
}

// elapsed formats a session's running time: "42s", "14m", "2h 5m", "3d 4h".
func elapsed(d time.Duration) string {
	d = max(d, 0)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d/time.Hour), int(d%time.Hour/time.Minute))
	default:
		return fmt.Sprintf("%dd %dh", int(d/(24*time.Hour)), int(d%(24*time.Hour)/time.Hour))
	}
}

// parseRef splits a card reference ("src/auth/session.go:L38-L52",
// "a.go:12", "a.go") into its path and first line (0 when absent).
func parseRef(ref string) (path string, line int) {
	ref = strings.TrimSpace(ref)
	i := strings.LastIndexByte(ref, ':')
	if i <= 0 {
		return ref, 0
	}
	rng := ref[i+1:]
	first := rng
	for _, sep := range []string{"-", "–"} {
		if j := strings.Index(rng, sep); j >= 0 {
			first = rng[:j]
			break
		}
	}
	first = strings.TrimPrefix(first, "L")
	n, err := strconv.Atoi(first)
	if err != nil || n < 0 {
		return ref, 0
	}
	return ref[:i], n
}

// lowerFirst lowercases the first letter of s unless it starts an acronym or
// identifier ("API", "tokenValid"), for joining summary items into one line.
func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 || r[0] < 'A' || r[0] > 'Z' {
		return s
	}
	if len(r) > 1 && (r[1] < 'a' || r[1] > 'z') {
		return s
	}
	r[0] += 'a' - 'A'
	return string(r)
}

// noun returns "1 card" or "3 cards".
func noun(n int, one, many string) string {
	return strconv.Itoa(n) + " " + plural(n, one, many)
}

// plural returns one for 1 and many otherwise.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
