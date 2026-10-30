// Package pathpolicy implements Layers 2 and 3 of docs/04-safety.md for file
// tools: writes outside the worktree are gated, credential stores are on a
// deny-read list, and settings files that could switch the safety layer off
// are protected.
//
// All path logic is parameterised by a GOOS string so the Windows rules (drive
// letters, UNC, case-insensitivity, both slashes) are testable on any host.
package pathpolicy

import (
	"path"
	"runtime"
	"strings"
)

// GuessOS returns "windows" when p looks like a Windows path (drive letter,
// UNC or device prefix) and the host's GOOS otherwise (or "linux" if the host
// is Windows and p is clearly POSIX).
func GuessOS(p string) string {
	if looksWindows(p) {
		return "windows"
	}
	if runtime.GOOS == "windows" {
		return "linux"
	}
	return runtime.GOOS
}

func looksWindows(p string) bool {
	if len(p) >= 2 && isLetter(p[0]) && p[1] == ':' {
		return true
	}
	return strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, `//?/`) || strings.HasPrefix(p, `//./`)
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

// foldsCase reports whether paths compare case-insensitively on goos.
func foldsCase(goos string) bool { return goos == "windows" || goos == "darwin" }

// splitVolume splits a slash-only Windows path into volume and rest. Volumes
// are "C:", "//server/share" or "" (rooted without a drive).
func splitVolume(p string) (vol, rest string) {
	lower := strings.ToLower(p)
	switch {
	case strings.HasPrefix(lower, "//?/unc/"), strings.HasPrefix(lower, "//./unc/"):
		p = "//" + p[8:]
	case strings.HasPrefix(p, "//?/"), strings.HasPrefix(p, "//./"), strings.HasPrefix(p, "/??/"):
		p = p[4:]
	}
	if len(p) >= 2 && isLetter(p[0]) && p[1] == ':' {
		return p[:2], p[2:]
	}
	if strings.HasPrefix(p, "//") && len(p) > 2 && p[2] != '/' {
		i := strings.IndexByte(p[2:], '/')
		if i < 0 {
			return p, ""
		}
		server := 2 + i
		j := strings.IndexByte(p[server+1:], '/')
		if j < 0 {
			return p, ""
		}
		end := server + 1 + j
		return p[:end], p[end:]
	}
	return "", p
}

// IsAbsOS reports whether p is absolute on goos. A Windows path rooted
// without a drive ("\x") counts as absolute (it is rooted on the current
// drive); a drive-relative path ("C:x") does not.
func IsAbsOS(goos, p string) bool {
	if goos != "windows" {
		return strings.HasPrefix(p, "/")
	}
	vol, rest := splitVolume(strings.ReplaceAll(p, `\`, "/"))
	if strings.HasPrefix(vol, "//") {
		return true
	}
	return strings.HasPrefix(rest, "/")
}

// CleanOS lexically cleans p using goos rules. Windows results use backslashes
// and keep their drive; trailing dots and spaces in components are dropped
// because Windows ignores them.
func CleanOS(goos, p string) string {
	if goos != "windows" {
		if p == "" {
			return "."
		}
		return path.Clean(p)
	}
	p = strings.ReplaceAll(p, `\`, "/")
	vol, rest := splitVolume(p)
	rooted := strings.HasPrefix(rest, "/")
	parts := strings.Split(rest, "/")
	kept := parts[:0]
	for _, c := range parts {
		if c != "." && c != ".." {
			c = strings.TrimRight(c, ". ")
		}
		if c != "" {
			kept = append(kept, c)
		}
	}
	joined := strings.Join(kept, "/")
	if rooted {
		joined = path.Clean("/" + joined)
	} else if joined != "" {
		joined = path.Clean(joined)
	}
	if vol != "" && joined == "" {
		joined = "/"
	}
	return strings.ReplaceAll(vol+joined, "/", `\`)
}

// JoinOS joins rel onto base using goos rules; an absolute rel wins. The
// result is cleaned.
func JoinOS(goos, base, rel string) string {
	if rel == "" {
		return CleanOS(goos, base)
	}
	if IsAbsOS(goos, rel) {
		if goos == "windows" {
			r := strings.ReplaceAll(rel, `\`, "/")
			if v, rest := splitVolume(r); v == "" && strings.HasPrefix(rest, "/") {
				bv, _ := splitVolume(strings.ReplaceAll(base, `\`, "/"))
				return CleanOS(goos, bv+rest)
			}
		}
		return CleanOS(goos, rel)
	}
	if goos == "windows" {
		r := strings.ReplaceAll(rel, `\`, "/")
		if v, rest := splitVolume(r); v != "" {
			bv, _ := splitVolume(strings.ReplaceAll(base, `\`, "/"))
			if !strings.EqualFold(v, bv) {
				return CleanOS(goos, v+"/"+rest)
			}
			rel = rest
		}
	}
	return CleanOS(goos, base+"/"+rel)
}

// KeyOS returns the canonical string used to compare paths on goos: cleaned,
// forward slashes, lower-cased where the file system folds case.
func KeyOS(goos, p string) string {
	c := CleanOS(goos, p)
	if goos == "windows" {
		c = strings.ReplaceAll(c, `\`, "/")
	}
	if foldsCase(goos) {
		c = strings.ToLower(c)
	}
	return c
}
