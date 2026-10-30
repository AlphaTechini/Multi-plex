package pathpolicy

import "strings"

// CheckWrite decides whether a file tool may write path without approval.
//
// A write is allowed only when every form of the path (as given and with
// symlinks resolved, so a link inside the worktree that points outside is
// caught) lies in the worktree or the writable allow-list, and none of them
// touches a protected settings file, a workspace protected path or a
// credential store.
func (p Policy) CheckWrite(path string) Verdict {
	cands, err := p.candidates(path)
	if err != nil {
		return Verdict{Rule: RuleUnresolvable, Reason: "the path could not be resolved safely", Path: path}
	}
	goos := p.os()
	shown := cands[0]
	for _, c := range cands {
		if p.matchSettings(goos, c) {
			return Verdict{Rule: RuleProtectedConfig, Reason: "this file can change or disable the safety layer", Path: shown}
		}
		if p.matchProtected(goos, c) {
			return Verdict{Rule: RuleProtectedPath, Reason: "this path is on the workspace protected list", Path: shown}
		}
		if p.matchDeny(goos, c) {
			return Verdict{Rule: RuleDenyRead, Reason: "this is a credential store", Path: shown}
		}
	}
	roots := append([]string{p.WorktreeRoot}, p.WritableExtra...)
	for _, c := range cands {
		if !p.inAnyRoot(goos, roots, c) {
			return Verdict{Rule: RuleOutsideWorktree, Reason: "the file is outside the worktree", Path: shown}
		}
	}
	return Verdict{Allowed: true, Path: shown}
}

// inAnyRoot reports whether c lies under one of roots, comparing against each
// root as written and with its own symlinks resolved.
func (p Policy) inAnyRoot(goos string, roots []string, c string) bool {
	for _, r := range roots {
		if r == "" {
			continue
		}
		er, ok := p.expandEntry(r)
		if !ok {
			continue
		}
		if WithinOS(goos, er, c) {
			return true
		}
		if rr, ok := p.resolvedEntry(er); ok && WithinOS(goos, rr, c) {
			return true
		}
	}
	return false
}

// expandEntry expands "~" and makes an entry absolute and clean.
func (p Policy) expandEntry(e string) (string, bool) {
	exp, ok := ExpandHome(p.os(), p.home(), e)
	if !ok || !IsAbsOS(p.os(), exp) {
		return "", false
	}
	return CleanOS(p.os(), exp), true
}

// resolvedEntry returns the symlink-resolved form of an absolute entry when
// resolution applies and changes it.
func (p Policy) resolvedEntry(e string) (string, bool) {
	if p.noResolve || p.os() != hostOS() {
		return "", false
	}
	r, err := resolveEntryCached(fromSlash(e))
	if err != nil || KeyOS(p.os(), r) == KeyOS(p.os(), e) {
		return "", false
	}
	return CleanOS(p.os(), r), true
}

// matchDeny reports whether c is under a deny-read entry.
func (p Policy) matchDeny(goos, c string) bool {
	for _, e := range p.DenyRead {
		ee, ok := p.expandEntry(e)
		if !ok {
			continue
		}
		if WithinOS(goos, ee, c) {
			return true
		}
		if rr, ok := p.resolvedEntry(ee); ok && WithinOS(goos, rr, c) {
			return true
		}
	}
	return false
}

// matchSettings reports whether c is a protected settings file or inside a
// protected settings directory.
func (p Policy) matchSettings(goos, c string) bool {
	ck := "/" + strings.TrimPrefix(KeyOS(goos, c), "/")
	for _, e := range p.ProtectedSettings {
		if exp, ok := ExpandHome(goos, p.home(), e); ok && IsAbsOS(goos, exp) {
			if WithinOS(goos, exp, c) {
				return true
			}
			continue
		}
		ek := strings.Trim(KeyOS(goos, strings.ReplaceAll(e, `\`, "/")), "/")
		if ek == "" || ek == "." {
			continue
		}
		if strings.HasSuffix(ck, "/"+ek) || strings.Contains(ck, "/"+ek+"/") {
			return true
		}
	}
	return false
}
