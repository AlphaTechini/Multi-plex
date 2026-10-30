package pathpolicy

import (
	"runtime"
	"strings"
)

// Within reports whether path is root or lies under it, using the host's path
// rules. A relative path is taken relative to root, so "../x" is outside.
// Matching is by whole components: C:\a does not contain C:\ab.
func Within(root, path string) bool { return WithinOS(runtime.GOOS, root, path) }

// WithinOS is Within with explicit GOOS rules ("windows" and "darwin" compare
// case-insensitively, "windows" accepts both slashes, drives and UNC).
func WithinOS(goos, root, path string) bool {
	if root == "" {
		return false
	}
	rk := KeyOS(goos, root)
	pk := KeyOS(goos, JoinOS(goos, root, path))
	if pk == rk {
		return true
	}
	if !strings.HasSuffix(rk, "/") {
		rk += "/"
	}
	return strings.HasPrefix(pk, rk)
}
