// Package envscrub removes credential variables from an environment before an
// agent process starts (docs/04-safety.md, Layer 3). Without the keys, remote
// side effects such as "aws s3 rm" or "git push" cannot happen silently.
package envscrub

import (
	"sort"
	"strconv"
	"strings"
)

// Options tune a scrub.
type Options struct {
	// Keep lists variables the agent CLI itself needs (exact names or globs,
	// for example its provider's API key). Callers (adapters) set this; it is
	// not a user setting. Keep overrides the built-in deny patterns, but never
	// ExtraDeny.
	Keep []string
	// ExtraDeny is the user-editable list of further names or globs to remove.
	// It always wins over Keep and over the built-in Keep list.
	ExtraDeny []string
}

// Result is a scrubbed environment plus what changed, for the UI. Only names
// are reported, never values.
type Result struct {
	Env     []string // the scrubbed environment, "NAME=value" entries
	Removed []string // names removed, sorted
	Added   []string // names added or overwritten by Multiplex, sorted
}

// Scrub returns env without known credential variables and with git
// credential helpers disabled. It is ScrubWith without a Keep list.
//
// Names are matched case-insensitively on every OS: on Windows because that is
// how the environment works, elsewhere because removing a lower-case
// "github_token" too is the safe direction.
func Scrub(env []string, extraDeny []string) []string {
	return ScrubWith(env, Options{ExtraDeny: extraDeny}).Env
}

// ScrubWith removes credential variables and applies the git hardening
// additions. Variables kept by builtin or caller Keep are only the agent CLI's
// own (ANTHROPIC_*, CLAUDE_* and whatever the adapter lists). All AWS_* are
// removed, including AWS_REGION: a region can live in the AWS config file, and
// docs/04 names AWS_* as a whole.
//
// Additions: GIT_CONFIG_COUNT/KEY/VALUE entries that set an empty
// credential.helper (which resets the helper list, so Git Credential Manager
// and the OS keychain helpers are not consulted), GIT_TERMINAL_PROMPT=0 and
// GCM_INTERACTIVE=never so git fails instead of asking for a password.
func ScrubWith(env []string, opts Options) Result {
	keep := append(append([]string(nil), builtinKeep...), opts.Keep...)
	var res Result
	removed := map[string]string{}
	out := make([]string, 0, len(env)+5)
	for _, e := range env {
		name := envName(e)
		up := strings.ToUpper(name)
		switch {
		case name == "":
			out = append(out, e)
		case matchAny(opts.ExtraDeny, up):
			removed[name] = name
		case matchAny(keep, up):
			out = append(out, e)
		case matchAny(denyPatterns, up):
			removed[name] = name
		default:
			out = append(out, e)
		}
	}
	out, added := hardenGit(out)
	res.Env = out
	res.Added = added
	for _, n := range removed {
		res.Removed = append(res.Removed, n)
	}
	sort.Strings(res.Removed)
	return res
}

// envName returns the variable name of a NAME=value entry. Windows keeps
// entries such as "=C:=C:\x" whose name starts with '='.
func envName(e string) string {
	start := 0
	if strings.HasPrefix(e, "=") {
		start = 1
	}
	i := strings.IndexByte(e[start:], '=')
	if i < 0 {
		return e
	}
	return e[:start+i]
}

// hardenGit disables git credential helpers and prompts for the child.
func hardenGit(env []string) (out []string, added []string) {
	count := 0
	for _, e := range env {
		if strings.EqualFold(envName(e), "GIT_CONFIG_COUNT") {
			if _, v, ok := strings.Cut(e[len(envName(e)):], "="); ok {
				if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
					count = n
				}
			}
		}
	}
	set := map[string]string{
		"GIT_CONFIG_COUNT":                strconv.Itoa(count + 1),
		"GIT_CONFIG_KEY_" + itoa(count):   "credential.helper",
		"GIT_CONFIG_VALUE_" + itoa(count): "",
		"GIT_TERMINAL_PROMPT":             "0",
		"GCM_INTERACTIVE":                 "never",
	}
	out = env[:0:0]
	for _, e := range env {
		n := strings.ToUpper(envName(e))
		if _, replace := set[n]; replace {
			continue
		}
		out = append(out, e)
	}
	for k, v := range set {
		out = append(out, k+"="+v)
		added = append(added, k)
	}
	sort.Strings(added)
	// Keep output order stable: additions sorted by name after the originals.
	sort.Strings(out[len(out)-len(set):])
	return out, added
}

func itoa(n int) string { return strconv.Itoa(n) }
