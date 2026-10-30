package diff

import "strings"

// Changed reports whether the lines an anchor points to changed: true when
// the exact snippet can no longer be found where the comment belongs. A
// snippet that merely moved (lines inserted above it) is not a change; an
// identical copy elsewhere with unrelated surroundings does not hide an edit
// of the original.
func Changed(a Anchor, newContent []string) bool {
	snip := strings.Split(a.Snippet, "\n")
	p, score, found := findSnippet(a, snip, newContent, false)
	if !found {
		return true
	}
	if p == a.Start-1 || len(a.Before)+len(a.After) == 0 {
		return false
	}
	return score == 0
}
