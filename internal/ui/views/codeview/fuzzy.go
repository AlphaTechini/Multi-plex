package codeview

import "strings"

// fuzzyMatch reports whether every rune of pattern appears in s in order
// (case-insensitively) and returns a score (higher is better) and the rune
// positions matched. Matches at the start of a path segment or file name,
// right after another match, and in the file name score higher; shorter
// paths win ties.
func fuzzyMatch(pattern, s string) (score int, pos []int, ok bool) {
	p := []rune(strings.ToLower(strings.ReplaceAll(pattern, " ", "")))
	if len(p) == 0 {
		return 0, nil, true
	}
	r := []rune(s)
	lower := []rune(strings.ToLower(s))
	if len(lower) != len(r) { // case folding changed the length; match the originals
		lower = r
	}
	base := 0
	for i, c := range r {
		if c == '/' {
			base = i + 1
		}
	}
	pos = make([]int, 0, len(p))
	pi, prev := 0, -2
	for i := 0; i < len(lower) && pi < len(p); i++ {
		if lower[i] != p[pi] {
			continue
		}
		switch {
		case i == prev+1:
			score += 8
		case i == 0 || r[i-1] == '/' || r[i-1] == '_' || r[i-1] == '-' || r[i-1] == '.':
			score += 6
		default:
			score++
		}
		if i >= base {
			score += 2
		}
		pos = append(pos, i)
		prev = i
		pi++
	}
	if pi < len(p) {
		return 0, nil, false
	}
	return score*4 - len(r)/8, pos, true
}
