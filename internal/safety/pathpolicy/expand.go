package pathpolicy

import "strings"

// ExpandHome expands a leading "~" using home. The second result is false for
// "~name" forms, which cannot be resolved here and must not be trusted. On
// Windows "~\x" works as well as "~/x".
func ExpandHome(goos, home, p string) (string, bool) {
	if !strings.HasPrefix(p, "~") {
		return p, true
	}
	if p == "~" {
		return home, home != ""
	}
	if p[1] == '/' || (goos == "windows" && p[1] == '\\') {
		if home == "" {
			return p, false
		}
		return home + "/" + p[2:], true
	}
	return p, false
}
