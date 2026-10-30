package kit

import (
	"iter"
	"strings"
)

// Lines splits s into lines on "\n". An empty string is one empty line, so
// the count always matches what the terminal shows.
func Lines(s string) []string { return strings.Split(s, "\n") }

// splitLines iterates over the lines of s without allocating a slice.
func splitLines(s string) iter.Seq[string] { return strings.SplitSeq(s, "\n") }
