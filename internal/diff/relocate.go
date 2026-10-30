package diff

import "strings"

// Relocate finds where an anchor's lines are in the new content (one entry
// per line, no line endings). It looks for the exact snippet first, then for
// a whitespace-insensitive match. When several places match, the one whose
// surrounding lines agree most with the stored context wins, then the one
// nearest to the old position. ok is false when the snippet is gone, which
// marks the comment outdated.
func Relocate(a Anchor, newContent []string) (start, end int, ok bool) {
	snip := strings.Split(a.Snippet, "\n")
	for _, fuzzy := range [2]bool{false, true} {
		if p, _, found := findSnippet(a, snip, newContent, fuzzy); found {
			return p + 1, p + len(snip), true
		}
	}
	return 0, 0, false
}

// findSnippet returns the 0-based start of the best match of snip in content
// and its context score.
func findSnippet(a Anchor, snip, content []string, fuzzy bool) (pos, score int, found bool) {
	n := len(snip)
	if n == 0 || len(content) < n {
		return 0, 0, false
	}
	old := a.Start - 1
	maxScore := len(a.Before) + len(a.After)
	bestDist := 0
	consider := func(p int) {
		if !matchAt(snip, content, p, fuzzy) {
			return
		}
		s := contextScore(a, content, p, n, fuzzy)
		d := p - old
		if d < 0 {
			d = -d
		}
		if !found || s > score || (s == score && d < bestDist) {
			pos, score, bestDist, found = p, s, d, true
		}
	}
	// Fast path: the lines are still where they were, with their context.
	if old >= 0 && old+n <= len(content) {
		consider(old)
		if found && score == maxScore {
			return pos, score, true
		}
	}
	for p := 0; p+n <= len(content); p++ {
		if p != old {
			consider(p)
		}
	}
	return pos, score, found
}

// matchAt reports whether content[p:p+len(snip)] equals snip.
func matchAt(snip, content []string, p int, fuzzy bool) bool {
	for i, s := range snip {
		if !lineEq(s, content[p+i], fuzzy) {
			return false
		}
	}
	return true
}

// contextScore counts the stored context lines that are still in place
// around a candidate position.
func contextScore(a Anchor, content []string, p, n int, fuzzy bool) int {
	score := 0
	for i := 1; i <= len(a.Before); i++ {
		if j := p - i; j >= 0 && lineEq(a.Before[len(a.Before)-i], content[j], fuzzy) {
			score++
		}
	}
	for i, s := range a.After {
		if j := p + n + i; j < len(content) && lineEq(s, content[j], fuzzy) {
			score++
		}
	}
	return score
}

// lineEq compares two lines exactly (ignoring a trailing carriage return) or,
// when fuzzy, ignoring all whitespace.
func lineEq(a, b string, fuzzy bool) bool {
	if !fuzzy {
		return strings.TrimSuffix(a, "\r") == strings.TrimSuffix(b, "\r")
	}
	i, j := 0, 0
	for {
		for i < len(a) && isSpace(a[i]) {
			i++
		}
		for j < len(b) && isSpace(b[j]) {
			j++
		}
		if i == len(a) || j == len(b) {
			return i == len(a) && j == len(b)
		}
		if a[i] != b[j] {
			return false
		}
		i++
		j++
	}
}
