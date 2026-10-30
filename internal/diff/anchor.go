package diff

import "strings"

// AnchorContext is how many lines of context NewAnchor stores on each side.
const AnchorContext = 3

// Anchor pins a review comment to lines of the new side of a file, so the
// comment can find its place again after the file changes (docs/07
// "Anchoring").
type Anchor struct {
	Path string // relative to the workspace root, slash-separated
	// Start and End are 1-based, inclusive line numbers on the new side.
	Start, End int
	// Snippet is the exact anchored lines joined by "\n".
	Snippet string
	// Before and After are up to AnchorContext lines around the snippet.
	Before, After []string
}

// NewAnchor builds an Anchor for lines start..end (1-based, inclusive) of
// content, one entry per line. Out-of-range lines are clamped.
func NewAnchor(path string, start, end int, content []string) Anchor {
	start = max(start, 1)
	end = min(end, len(content))
	a := Anchor{Path: path, Start: start, End: end}
	if start > end {
		return a
	}
	a.Snippet = strings.Join(content[start-1:end], "\n")
	a.Before = append([]string(nil), content[max(0, start-1-AnchorContext):start-1]...)
	a.After = append([]string(nil), content[end:min(len(content), end+AnchorContext)]...)
	return a
}
