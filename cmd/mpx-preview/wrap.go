package main

import "github.com/charmbracelet/x/ansi"

// ansiWrap word-wraps plain text to w cells.
func ansiWrap(s string, w int) string {
	if w <= 0 {
		return s
	}
	return ansi.Wrap(s, w, "")
}
