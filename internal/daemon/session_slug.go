package daemon

import (
	"strconv"
	"strings"
)

// maxSlug is the longest slug.
const maxSlug = 40

// slugify turns free text into a lowercase [a-z0-9-] slug of at most 40
// characters, cut at a word boundary where it can be. Empty input gives "session".
func slugify(s string) string {
	var b strings.Builder
	dash := true
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > maxSlug {
		cut := out[:maxSlug]
		if i := strings.LastIndexByte(cut, '-'); i >= maxSlug/2 && out[maxSlug] != '-' {
			cut = cut[:i]
		}
		out = strings.Trim(cut, "-")
	}
	if out == "" {
		return "session"
	}
	return out
}

// uniqueSlug returns base, or base-2, base-3, ... until taken(slug) is false.
// The result still fits in 40 characters.
func uniqueSlug(base string, taken func(string) bool) string {
	if !taken(base) {
		return base
	}
	for n := 2; ; n++ {
		suffix := "-" + strconv.Itoa(n)
		root := base
		if len(root)+len(suffix) > maxSlug {
			root = strings.TrimRight(root[:maxSlug-len(suffix)], "-")
		}
		if cand := root + suffix; !taken(cand) {
			return cand
		}
	}
}
