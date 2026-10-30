package classify

import (
	"os"
	"strings"

	"github.com/alphatechini/multi-plex/internal/safety/pathpolicy"
)

// cctx is a Context with defaults applied and the path style decided.
type cctx struct {
	Context
	goos      string
	root      string
	home      string
	protected []string
	fl        flavour
	tmpVars   map[string]bool     // variables assigned from mktemp in this command
	loopVars  map[string][]string // for-loop variables and the words they range over
}

// tempDir is where $TMPDIR, $TEMP and mktemp results live; deleting inside
// it is not gated. Empty when unknown.
func (x *cctx) tempDir() string {
	for _, d := range x.SafeDirs {
		if d != "" {
			return d
		}
	}
	return ""
}

func newCctx(c Context) cctx {
	x := cctx{Context: c, root: c.WorktreeRoot, fl: flavourOf(c.Shell)}
	if x.Cwd == "" {
		x.Cwd = x.root
	}
	probe := x.root
	if probe == "" {
		probe = x.Cwd
	}
	x.goos = pathpolicy.GuessOS(probe)
	x.home = c.Home
	if x.home == "" {
		x.home, _ = os.UserHomeDir()
	}
	x.protected = c.ProtectedBranches
	if len(x.protected) == 0 {
		x.protected = []string{"main", "master"}
	}
	return x
}

func (x *cctx) isProtectedBranch(b string) bool {
	for _, p := range x.protected {
		if p == b && b != "" {
			return true
		}
	}
	return false
}

var homeVars = []string{
	"${home}", "$home", "$env:home", "$env:userprofile", "${userprofile}", "$userprofile",
	"%userprofile%", "%home%", "%homedrive%%homepath%", "%homepath%", "$env:homepath",
}

// homeSubVars are Windows variables that default to a folder under the home
// directory.
var homeSubVars = []struct{ name, sub string }{
	{"$env:localappdata", "/AppData/Local"}, {"%localappdata%", "/AppData/Local"},
	{"$env:appdata", "/AppData/Roaming"}, {"%appdata%", "/AppData/Roaming"},
	{"$env:temp", "/AppData/Local/Temp"}, {"%temp%", "/AppData/Local/Temp"},
	{"$env:tmp", "/AppData/Local/Temp"}, {"%tmp%", "/AppData/Local/Temp"},
}

var cwdVars = []string{"$(pwd)", "`pwd`", "${pwd}", "$pwd", "$env:pwd", "%cd%", "$(get-location)", "(get-location)"}

// resolve turns a command-line path into an absolute cleaned path without
// touching the file system. ok is false when the path depends on something
// unknown (a variable, a command substitution, an unknown working directory
// or a PowerShell provider path), which callers treat as "outside".
func (x *cctx) resolve(tok, cwd string, cwdKnown bool) (abs string, ok bool) {
	t := tok
	lower := strings.ToLower(t)
	for _, v := range homeVars {
		if strings.HasPrefix(lower, v) && sepOrEnd(t[len(v):]) {
			if x.home == "" {
				return "", false
			}
			t = x.home + t[len(v):]
			lower = strings.ToLower(t)
			break
		}
	}
	for _, v := range homeSubVars {
		if strings.HasPrefix(lower, v.name) && sepOrEnd(t[len(v.name):]) {
			if x.home == "" {
				return "", false
			}
			t = x.home + v.sub + t[len(v.name):]
			lower = strings.ToLower(t)
			break
		}
	}
	for _, v := range cwdVars {
		if strings.HasPrefix(lower, v) && sepOrEnd(t[len(v):]) {
			if !cwdKnown {
				return "", false
			}
			t = cwd + t[len(v):]
			break
		}
	}
	if v, rest, ok := x.tempVar(t); ok {
		if v == "" {
			return "", false
		}
		t = v + rest
	}
	exp, good := pathpolicy.ExpandHome(x.goos, x.home, t)
	if !good {
		return "", false
	}
	t = exp
	if i := dynamicIndex(t, x.goos); i >= 0 {
		if i == 0 {
			return "", false
		}
		t = t[:i]
	}
	if x.goos == "windows" && providerPath(t) {
		return "", false
	}
	if pathpolicy.IsAbsOS(x.goos, t) {
		base := cwd
		if !cwdKnown {
			base = x.root
		}
		return pathpolicy.JoinOS(x.goos, base, t), true
	}
	if !cwdKnown || cwd == "" {
		return "", false
	}
	return pathpolicy.JoinOS(x.goos, cwd, t), true
}

// tempVar recognises $TMPDIR, $TEMP, $TMP and variables assigned from mktemp
// at the start of a path and returns the temp directory they stand for.
func (x *cctx) tempVar(t string) (dir, rest string, ok bool) {
	if len(t) < 2 || t[0] != '$' {
		return "", "", false
	}
	name, rest := t[1:], ""
	if name[0] == '{' {
		end := strings.IndexByte(name, '}')
		if end < 0 {
			return "", "", false
		}
		name, rest = name[1:end], name[end+1:]
	} else {
		end := 0
		for end < len(name) && (isAlpha(name[end]) || name[end] == '_' || (end > 0 && name[end] >= '0' && name[end] <= '9')) {
			end++
		}
		name, rest = name[:end], name[end:]
	}
	if !sepOrEnd(rest) {
		return "", "", false
	}
	if x.tmpVars[name] {
		// mktemp creates a new entry inside the temp directory.
		if d := x.tempDir(); d != "" {
			return d + "/mktemp.x", rest, true
		}
		return "", rest, true
	}
	switch strings.ToUpper(name) {
	case "TMPDIR", "TEMP", "TMP":
		return x.tempDir(), rest, true
	}
	return "", "", false
}

func sepOrEnd(rest string) bool { return rest == "" || rest[0] == '/' || rest[0] == '\\' }

// dynamicIndex finds the first character that makes a path unknowable.
func dynamicIndex(t, goos string) int {
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case '$', '`':
			return i
		case '%':
			if goos == "windows" && strings.IndexByte(t[i+1:], '%') >= 0 {
				return i
			}
		case '(':
			return i
		}
	}
	return -1
}

// providerPath reports PowerShell provider paths such as HKLM:\Software.
func providerPath(t string) bool {
	i := strings.IndexByte(t, ':')
	if i < 2 || i > 12 {
		return false
	}
	for j := 0; j < i; j++ {
		if !isAlpha(t[j]) && t[j] != '_' {
			return false
		}
	}
	return !strings.HasPrefix(t[i+1:], "//")
}

// inside reports whether abs is in the worktree, or strictly inside a safe
// directory (deleting the temp directory itself is not safe).
func (x *cctx) inside(abs string) bool {
	if x.root != "" && pathpolicy.WithinOS(x.goos, x.root, abs) {
		return true
	}
	for _, d := range x.SafeDirs {
		if d == "" {
			continue
		}
		if pathpolicy.KeyOS(x.goos, d) != pathpolicy.KeyOS(x.goos, abs) && pathpolicy.WithinOS(x.goos, d, abs) {
			return true
		}
	}
	return false
}

var slashFlagCmds = setOf("rmdir", "rd", "del", "erase", "format", "diskpart", "xcopy", "robocopy", "attrib", "cipher", "icacls", "takeown")

func isSlashFlag(a string) bool {
	if len(a) < 2 || a[0] != '/' {
		return false
	}
	if len(a) == 2 {
		return isAlpha(a[1]) || a[1] == '?'
	}
	return a[2] == ':' && isAlpha(a[1])
}

// operandsOf returns the path-like arguments of a simple command: every
// argument that is not a flag. find is special: only its leading start paths.
func operandsOf(s *Simple, name string, fl flavour) []string {
	args := s.Argv[1:]
	if name == "find" {
		var out []string
		for _, a := range args {
			if len(out) == 0 && (a == "-H" || a == "-L" || a == "-P") {
				continue
			}
			if a == "" || a[0] == '-' || a == "(" || a == "!" || a == ")" || a == "," {
				break
			}
			out = append(out, a)
		}
		return out
	}
	slash := slashFlagCmds[name] || fl == flCmd
	var out []string
	dd := false
	for _, a := range args {
		if !dd && a == "--" {
			dd = true
			continue
		}
		if !dd && len(a) > 1 && a[0] == '-' {
			if s.Fold {
				if k := strings.IndexByte(a, ':'); k > 1 {
					switch strings.ToLower(a[1:k]) {
					case "path", "literalpath", "destination", "p", "lp":
						out = append(out, splitCommas(a[k+1:], true)...)
					}
				}
			}
			continue
		}
		if !dd && slash && isSlashFlag(a) {
			continue
		}
		out = append(out, splitCommas(a, s.Fold)...)
	}
	return out
}

func splitCommas(a string, ps bool) []string {
	if ps && strings.IndexByte(a, ',') >= 0 {
		return strings.Split(a, ",")
	}
	return []string{a}
}

// outside reports whether a command works on something outside the worktree.
// Operands come from the command line and, for commands fed by a pipe or
// xargs, from the previous command (so "find / | xargs rm" is caught).
func (x *cctx) outside(s *Simple, ops []string) bool {
	if len(s.PrevArgv) > 0 && (s.ViaXargs || (s.Piped && len(ops) == 0)) {
		prev := &Simple{Argv: s.PrevArgv, Fold: s.Fold}
		ops = append(ops[:len(ops):len(ops)], operandsOf(prev, cmdName(s.PrevArgv[0]), x.fl)...)
	}
	if len(ops) == 0 {
		ops = []string{"."}
	}
	ops = x.expandLoopVars(ops)
	for _, op := range ops {
		abs, ok := x.resolve(op, s.Cwd, s.CwdKnown)
		if !ok || !x.inside(abs) {
			return true
		}
	}
	return false
}

// pathOutside reports whether a single path (from a regex capture) is outside.
func (x *cctx) pathOutside(p string, s *Simple) bool {
	abs, ok := x.resolve(p, s.Cwd, s.CwdKnown)
	return !ok || !x.inside(abs)
}

// expandLoopVars replaces "$f" (f a for-loop variable) by the words the loop
// ranges over, so "for f in *.log; do rm "$f"; done" is checked on *.log.
func (x *cctx) expandLoopVars(ops []string) []string {
	if len(x.loopVars) == 0 {
		return ops
	}
	var out []string
	for _, op := range ops {
		name, rest, ok := varPrefix(op)
		items, loop := x.loopVars[name]
		if !ok || !loop {
			out = append(out, op)
			continue
		}
		for _, it := range items {
			out = append(out, it+rest)
		}
	}
	return out
}

// varPrefix splits "$name/rest" or "${name}/rest".
func varPrefix(t string) (name, rest string, ok bool) {
	if len(t) < 2 || t[0] != '$' {
		return "", "", false
	}
	n := t[1:]
	if n[0] == '{' {
		end := strings.IndexByte(n, '}')
		if end < 0 {
			return "", "", false
		}
		return n[1:end], n[end+1:], sepOrEnd(n[end+1:])
	}
	end := 0
	for end < len(n) && (isAlpha(n[end]) || n[end] == '_' || (end > 0 && n[end] >= '0' && n[end] <= '9')) {
		end++
	}
	return n[:end], n[end:], end > 0 && sepOrEnd(n[end:])
}
