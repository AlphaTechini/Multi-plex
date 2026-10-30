package classify

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ArgvPatterns holds one or more argv patterns. In JSON it is either a single
// pattern (["git","push","--force"]) or a list of patterns.
//
// A pattern is a prefix match on the command and its arguments. The first
// element names the command (matched case-insensitively against the base name
// without directory or .exe). Every element is a list of alternatives
// separated by "|", and each alternative is one of:
//
//	text         the token equals text (case-sensitive; see (?i))
//	a*b?         glob: * is any run of characters, ? any one character
//	-[abc]       a short-flag cluster containing any of a, b or c (-f, -fu, -rf)
//	$branch      a protected branch, also as +branch, src:branch or refs/heads/branch
//	(?i)text     prefix: compare this element case-insensitively
//
// The element "**" matches any number of tokens (including none), so
// ["git","push","**","--force"] finds the flag anywhere after "push".
type ArgvPatterns [][]string

// UnmarshalJSON accepts a single pattern or a list of patterns.
func (a *ArgvPatterns) UnmarshalJSON(b []byte) error {
	var one []string
	if err := json.Unmarshal(b, &one); err == nil {
		*a = ArgvPatterns{one}
		return nil
	}
	var many [][]string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("argv must be a list of strings or a list of such lists: %w", err)
	}
	*a = many
	return nil
}

type altKind int

const (
	altLiteral altKind = iota
	altGlob
	altCluster
	altBranch
)

type alt struct {
	kind altKind
	text string
}

type elem struct {
	alts  []alt
	star2 bool
	fold  bool
}

type pattern struct {
	elems  []elem
	cmds   []string // literal command names, empty when the first element has a glob
	hasGlb bool
}

func compilePattern(raw []string) (pattern, error) {
	if len(raw) == 0 {
		return pattern{}, fmt.Errorf("empty argv pattern")
	}
	var p pattern
	for i, r := range raw {
		var e elem
		if r == "**" {
			if i == 0 {
				return p, fmt.Errorf("argv pattern cannot start with **")
			}
			e.star2 = true
			p.elems = append(p.elems, e)
			continue
		}
		if strings.HasPrefix(r, "(?i)") {
			e.fold = true
			r = r[4:]
		}
		if i == 0 {
			e.fold = true
		}
		for _, a := range strings.Split(r, "|") {
			if a == "" {
				return p, fmt.Errorf("empty alternative in %q", raw[i])
			}
			if i == 0 {
				a = cmdName(a) // "./deploy.sh" matches the command word the same way
			}
			if e.fold {
				a = strings.ToLower(a)
			}
			switch {
			case a == "$branch":
				e.alts = append(e.alts, alt{kind: altBranch})
			case strings.HasPrefix(a, "-[") && strings.HasSuffix(a, "]") && len(a) > 3:
				e.alts = append(e.alts, alt{kind: altCluster, text: a[2 : len(a)-1]})
			case strings.ContainsAny(a, "*?"):
				e.alts = append(e.alts, alt{kind: altGlob, text: a})
			default:
				e.alts = append(e.alts, alt{kind: altLiteral, text: a})
			}
		}
		p.elems = append(p.elems, e)
	}
	for _, a := range p.elems[0].alts {
		if a.kind == altLiteral {
			p.cmds = append(p.cmds, a.text)
		} else {
			p.hasGlb = true
		}
	}
	return p, nil
}

// matchEnv carries what the matcher needs besides the tokens.
type matchEnv struct {
	fold      bool // flags are case-insensitive (PowerShell)
	protected []string
}

// match reports whether the pattern matches the command name and arguments.
func (p pattern) match(cmd string, args []string, env *matchEnv) bool {
	if !p.elems[0].matches(cmd, true, env) {
		return false
	}
	return matchRest(p.elems[1:], args, env)
}

func matchRest(es []elem, args []string, env *matchEnv) bool {
	if len(es) == 0 {
		return true
	}
	if es[0].star2 {
		for k := 0; k <= len(args); k++ {
			if matchRest(es[1:], args[k:], env) {
				return true
			}
		}
		return false
	}
	if len(args) == 0 || !es[0].matches(args[0], env.fold, env) {
		return false
	}
	return matchRest(es[1:], args[1:], env)
}

func (e elem) matches(tok string, fold bool, env *matchEnv) bool {
	fold = fold || e.fold
	if fold {
		tok = strings.ToLower(tok)
	}
	for _, a := range e.alts {
		switch a.kind {
		case altLiteral:
			if tok == a.text {
				return true
			}
		case altGlob:
			if glob(a.text, tok) {
				return true
			}
		case altCluster:
			if len(tok) > 1 && tok[0] == '-' && tok[1] != '-' && strings.ContainsAny(tok[1:], a.text) {
				return true
			}
		case altBranch:
			if isProtectedRef(tok, env.protected) {
				return true
			}
		}
	}
	return false
}

// isProtectedRef reports whether a git push argument names a protected
// branch: "main", "+main", "HEAD:main", "x:refs/heads/main".
func isProtectedRef(tok string, protected []string) bool {
	t := strings.TrimPrefix(tok, "+")
	if i := strings.LastIndexByte(t, ':'); i >= 0 {
		t = t[i+1:]
	}
	t = strings.TrimPrefix(t, "refs/heads/")
	for _, b := range protected {
		if t == b {
			return true
		}
	}
	return false
}

// glob matches s against a pattern where * is any run and ? any one byte.
func glob(pat, s string) bool {
	px, sx := 0, 0
	star, mark := -1, 0
	for sx < len(s) {
		switch {
		case px < len(pat) && (pat[px] == '?' || pat[px] == s[sx]):
			px++
			sx++
		case px < len(pat) && pat[px] == '*':
			star, mark = px, sx
			px++
		case star >= 0:
			px = star + 1
			mark++
			sx = mark
		default:
			return false
		}
	}
	for px < len(pat) && pat[px] == '*' {
		px++
	}
	return px == len(pat)
}
