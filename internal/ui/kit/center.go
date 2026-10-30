package kit

import "github.com/charmbracelet/x/ansi"

// Center returns a single line exactly w cells wide with s centred in it
// (an odd leftover space goes on the right). Longer lines are truncated.
func Center(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = Truncate(s, w, "…")
	left := (w - ansi.StringWidth(s)) / 2
	return PadRight(spaces(left)+s, w)
}
