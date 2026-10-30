package spec

import "strings"

// validSlug reports whether slug names a single folder inside the specs directory.
func validSlug(slug string) bool {
	if slug == "" || strings.HasPrefix(slug, ".") {
		return false
	}
	return !strings.ContainsAny(slug, "/\\:\x00") && !strings.Contains(slug, "..")
}
