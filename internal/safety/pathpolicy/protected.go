package pathpolicy

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func hostOS() string { return runtime.GOOS }

func fromSlash(p string) string { return filepath.FromSlash(p) }

// matchProtected reports whether c matches a workspace protected path.
func (p Policy) matchProtected(goos, c string) bool {
	for _, e := range p.Protected {
		if e == "" {
			continue
		}
		if strings.ContainsAny(e, "*?[") {
			if p.globMatch(goos, e, c) {
				return true
			}
			continue
		}
		exp, ok := ExpandHome(goos, p.home(), e)
		if !ok {
			continue
		}
		var abs string
		if IsAbsOS(goos, exp) {
			abs = exp
		} else if p.WorktreeRoot != "" {
			abs = JoinOS(goos, p.WorktreeRoot, exp)
		} else {
			continue
		}
		if WithinOS(goos, abs, c) {
			return true
		}
	}
	return false
}

// globMatch matches a glob entry against c: against the worktree-relative
// path when the pattern has a slash, otherwise against any base name inside
// the worktree.
func (p Policy) globMatch(goos, pattern, c string) bool {
	if p.WorktreeRoot == "" || !WithinOS(goos, p.WorktreeRoot, c) {
		return false
	}
	pat := strings.TrimPrefix(strings.ReplaceAll(pattern, `\`, "/"), "./")
	if foldsCase(goos) {
		pat = strings.ToLower(pat)
	}
	ck := KeyOS(goos, c)
	rk := strings.TrimSuffix(KeyOS(goos, p.WorktreeRoot), "/")
	rel := strings.TrimPrefix(strings.TrimPrefix(ck, rk), "/")
	if !strings.Contains(pat, "/") {
		for _, seg := range strings.Split(rel, "/") {
			if ok, _ := path.Match(pat, seg); ok {
				return true
			}
		}
		return false
	}
	pat = strings.TrimSuffix(pat, "/")
	// Match the pattern against every leading prefix of the path so that a
	// directory pattern protects everything below it.
	parts := strings.Split(rel, "/")
	for i := 1; i <= len(parts); i++ {
		if ok, _ := path.Match(pat, strings.Join(parts[:i], "/")); ok {
			return true
		}
	}
	return false
}
