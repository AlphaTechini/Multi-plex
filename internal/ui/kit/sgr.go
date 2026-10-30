package kit

import (
	"strings"
)

// rewriteSGR copies s, passing the parameter string of every SGR sequence
// ("\x1b[...m") to fn and writing fn's result in its place. Text and other
// escape sequences are copied unchanged.
func rewriteSGR(s string, fn func(params string) string) string {
	if !strings.Contains(s, "\x1b[") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 32)
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		j := i + 2
		for j < len(s) && (s[j] < 0x40 || s[j] > 0x7E) {
			j++
		}
		if j >= len(s) {
			b.WriteString(s[i:])
			return b.String()
		}
		if s[j] == 'm' {
			b.WriteString(fn(s[i+2 : j]))
		} else {
			b.WriteString(s[i : j+1])
		}
		s = s[j+1:]
	}
}

// sgrParams splits an SGR parameter string. An empty string is a reset.
func sgrParams(p string) []string {
	if p == "" {
		return []string{"0"}
	}
	return strings.Split(p, ";")
}

// colorArgs returns how many parameters after index i belong to an extended
// colour introducer (38, 48 or 58) at i: 2 for "5;n", 4 for "2;r;g;b".
func colorArgs(parts []string, i int) int {
	if i+1 >= len(parts) {
		return 0
	}
	switch parts[i+1] {
	case "5":
		return 2
	case "2":
		return 4
	}
	return 0
}
