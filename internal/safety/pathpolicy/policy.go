package pathpolicy

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Rule ids reported in a Verdict.
const (
	RuleOutsideWorktree = "write-outside-worktree" // Layer 2
	RuleProtectedConfig = "protected-settings"     // Layer 2: files that could disable the safety layer
	RuleProtectedPath   = "protected-path"         // Layer 1 gap: workspace protected paths
	RuleDenyRead        = "deny-read"              // Layer 3: credential stores
	RuleUnresolvable    = "unresolvable-path"      // path could not be resolved safely
)

// Policy is the data behind Layers 2 and 3 for file tools. All lists hold
// absolute paths, "~"-prefixed paths, or (for Protected and ProtectedSettings)
// relative patterns described on each field. A zero Policy has no worktree
// and gates every write.
type Policy struct {
	// WorktreeRoot is the session's worktree. Writes inside it need no
	// approval (Layer 1 makes them reversible).
	WorktreeRoot string
	// WritableExtra are further writable directories: temp and tool caches.
	WritableExtra []string
	// DenyRead are credential stores; reading or writing under them is gated.
	DenyRead []string
	// Protected are workspace "protected paths" (docs/04 Layer 1 known gap),
	// for example ".env" or "data/*.db". Relative entries are relative to the
	// worktree; entries with * ? [ are globs matched against the
	// worktree-relative path (or the base name when the entry has no slash).
	Protected []string
	// ProtectedSettings are files that can disable or reroute the safety
	// layer: Claude Code settings, the Multiplex safety rules file. Absolute
	// entries match that path (and anything below it); relative entries such
	// as ".claude/settings.json" match at any depth.
	ProtectedSettings []string
	// Home is the user's home directory, for "~" expansion.
	Home string

	goos      string
	noResolve bool
}

// Verdict is the answer to a path check.
type Verdict struct {
	Allowed bool
	Rule    string // one of the Rule constants when not allowed
	Reason  string // human-readable, safe to show (never file contents)
	Path    string // the normalised absolute path that was checked
}

// WithGOOS returns a copy that applies the path rules of goos instead of the
// host's. It exists so Windows and macOS logic can be tested anywhere; symlink
// resolution is skipped when goos differs from the host.
func (p Policy) WithGOOS(goos string) Policy { p.goos = goos; return p }

// WithoutSymlinks returns a copy that never touches the file system. The gate
// uses it for paths scraped from shell commands, where resolution is
// best-effort anyway and cost matters.
func (p Policy) WithoutSymlinks() Policy { p.noResolve = true; return p }

// OS is the GOOS whose path rules the policy applies.
func (p Policy) OS() string { return p.os() }

func (p Policy) os() string {
	if p.goos != "" {
		return p.goos
	}
	return runtime.GOOS
}

func (p Policy) home() string {
	if p.Home != "" {
		return p.Home
	}
	h, _ := os.UserHomeDir()
	return h
}

// Abs returns path as a cleaned absolute path using the policy's rules:
// "~" is expanded and relative paths are taken relative to the worktree.
func (p Policy) Abs(path string) (string, error) {
	joined, err := p.join(path)
	if err != nil {
		return "", err
	}
	return CleanOS(p.os(), joined), nil
}

// join expands and joins without cleaning, preserving ".." for the resolver.
func (p Policy) join(path string) (string, error) {
	goos := p.os()
	if path == "" || strings.ContainsRune(path, 0) {
		return "", errors.New("empty or invalid path")
	}
	exp, ok := ExpandHome(goos, p.home(), path)
	if !ok {
		return "", errors.New("cannot expand " + path)
	}
	if IsAbsOS(goos, exp) {
		if goos == "windows" {
			r := strings.ReplaceAll(exp, `\`, "/")
			if v, rest := splitVolume(r); v == "" && strings.HasPrefix(rest, "/") && p.WorktreeRoot != "" {
				bv, _ := splitVolume(strings.ReplaceAll(p.WorktreeRoot, `\`, "/"))
				return bv + rest, nil
			}
		}
		return exp, nil
	}
	if p.WorktreeRoot == "" {
		return "", errors.New("relative path without a worktree")
	}
	return p.WorktreeRoot + "/" + exp, nil
}

// candidates returns the lexical absolute path and, when it differs, the
// symlink-resolved one. Both must pass every check.
func (p Policy) candidates(path string) ([]string, error) {
	joined, err := p.join(path)
	if err != nil {
		return nil, err
	}
	goos := p.os()
	lex := CleanOS(goos, joined)
	out := []string{lex}
	if p.noResolve || goos != runtime.GOOS {
		return out, nil
	}
	res, err := resolveSymlinks(filepath.FromSlash(joined))
	if err != nil {
		return nil, err
	}
	if KeyOS(goos, res) != KeyOS(goos, lex) {
		out = append(out, CleanOS(goos, res))
	}
	return out, nil
}

// Display returns path relative to the worktree, slash-separated, when it is
// inside; otherwise the absolute path, slash-separated.
func (p Policy) Display(path string) string {
	abs, err := p.Abs(path)
	if err != nil {
		return strings.ReplaceAll(path, `\`, "/")
	}
	goos := p.os()
	if p.WorktreeRoot != "" && WithinOS(goos, p.WorktreeRoot, abs) {
		ac := strings.ReplaceAll(abs, `\`, "/")
		rc := strings.TrimSuffix(strings.ReplaceAll(CleanOS(goos, p.WorktreeRoot), `\`, "/"), "/")
		if len(ac) <= len(rc) {
			return "."
		}
		return strings.TrimPrefix(ac[len(rc):], "/")
	}
	return strings.ReplaceAll(abs, `\`, "/")
}
