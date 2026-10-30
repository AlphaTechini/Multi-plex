// Package classify is Layer 4 of docs/04-safety.md: a fast first filter that
// sorts a shell command into safe, dangerous or unknown.
//
// It is only a first filter. A command such as
//
//	python -c "import shutil; shutil.rmtree(path)"
//
// with a computed path cannot be understood by looking at text, so it is
// classified unknown and left to Layers 1 to 3 (checkpoints, write gating,
// withheld credentials). Obvious forms with literal paths are caught by
// regex rules, and one-liner interpreters that run shell strings
// (os.system, subprocess, execSync) have those strings classified.
//
// Rules are data (default_rules.json, extended by user files, see Load).
// Compound commands are split on &&, ||, ;, |, newlines and PowerShell
// separators; env assignments and wrappers (sudo, env, command, xargs,
// bash -c, pwsh -Command, cmd /c, ssh host ...) are looked through; the most
// dangerous part decides. The classifier performs no file system access.
package classify

import "strings"

// Classify classifies cmd with the default rules.
func Classify(cmd string, ctx Context) Result { return Default().Classify(cmd, ctx) }

// Classify classifies cmd. The most dangerous part of a compound command
// decides; a command is safe only if every part is safe.
func (r *Rules) Classify(cmd string, ctx Context) Result {
	return r.Analyze(cmd, ctx, false).Result
}

// Analysis is a classification together with the parsed commands.
type Analysis struct {
	Result Result
	// Commands are the simple commands visited (up to and including the one
	// that was found dangerous). Only filled when requested.
	Commands []Simple
	// Resolver resolves command-line paths under the same context.
	Resolver *Resolver
}

// Analyze classifies cmd like Classify and, when withCommands is set, also
// returns the parsed simple commands so the gate can inspect their paths
// without parsing twice.
func (r *Rules) Analyze(cmd string, ctx Context, withCommands bool) Analysis {
	w := newWalker(ctx)
	var cmds []Simple
	var danger *Result
	var firstSafe *Result
	allSafe := true
	seen := 0
	w.visitText = func(text string, st *wstate) bool {
		if res, ok := r.evalFull(w, text, st); ok {
			danger = &res
			return true
		}
		return false
	}
	w.visit = func(s *Simple) bool {
		seen++
		if withCommands {
			cmds = append(cmds, *s)
		}
		res := r.evalSimple(w, s)
		switch res.Class {
		case Dangerous:
			danger = &res
			return true
		case Safe:
			if firstSafe == nil {
				firstSafe = &res
			}
		default:
			allSafe = false
		}
		return false
	}
	w.run(cmd)
	a := Analysis{Commands: cmds, Resolver: &Resolver{x: w.cx}}
	switch {
	case danger != nil:
		a.Result = *danger
	case w.tooComplex:
		a.Result = Result{Class: Dangerous, RuleID: "command-too-complex",
			Reason: "the command is too large or deeply nested to check; it is gated to be safe"}
	case seen > 0 && allSafe && firstSafe != nil:
		a.Result = *firstSafe
	default:
		a.Result = Result{Class: Unknown}
	}
	return a
}

func resultOf(rule *Rule) Result {
	return Result{Class: rule.Class, RuleID: rule.ID, Reason: rule.Description, Kind: rule.Kind}
}

// evalFull applies full-scope regex rules to one nesting level's text.
func (r *Rules) evalFull(w *walker, text string, st *wstate) (Result, bool) {
	if len(r.full) == 0 {
		return Result{}, false
	}
	norm := strings.Join(strings.Fields(text), " ")
	for _, rule := range r.full {
		if rule.Class != Dangerous {
			continue
		}
		if rule.OnProtectedCurrentBranch && !w.cx.isProtectedBranch(w.cx.CurrentBranch) {
			continue
		}
		s := &Simple{Cwd: st.cwd, CwdKnown: st.known}
		if regexMatches(w, rule, norm, s) {
			return resultOf(rule), true
		}
	}
	return Result{}, false
}

// evalSimple classifies one simple command.
func (r *Rules) evalSimple(w *walker, s *Simple) Result {
	name := cmdName(s.Argv[0])
	env := &matchEnv{fold: s.Fold, protected: w.cx.protected}
	var safe *Rule
	try := func(list []*Rule) *Rule {
		for _, rule := range list {
			if !ruleMatches(w, rule, s, name, env) {
				continue
			}
			if rule.Class == Dangerous {
				return rule
			}
			if safe == nil && !writesFiles(s) {
				safe = rule
			}
		}
		return nil
	}
	if d := try(r.byCmd[name]); d != nil {
		return resultOf(d)
	}
	if d := try(r.anyCmd); d != nil {
		return resultOf(d)
	}
	if d := try(r.seg); d != nil {
		return resultOf(d)
	}
	if safe != nil {
		return resultOf(safe)
	}
	return Result{Class: Unknown}
}

// writesFiles reports whether the command redirects output to a real file,
// which disqualifies it from being "known safe".
func writesFiles(s *Simple) bool {
	for _, r := range s.Redirects {
		if r.IsOutput() && !r.IsSink() {
			return true
		}
	}
	return false
}

func ruleMatches(w *walker, rule *Rule, s *Simple, name string, env *matchEnv) bool {
	if rule.Class == Safe && rule.c.full {
		return false
	}
	if rule.OnProtectedCurrentBranch && !w.cx.isProtectedBranch(w.cx.CurrentBranch) {
		return false
	}
	if len(rule.c.patterns) > 0 {
		matched := false
		for _, p := range rule.c.patterns {
			if p.match(name, s.Argv[1:], env) {
				matched = true
				break
			}
		}
		if !matched || excluded(rule, name, s, env) {
			return false
		}
		if rule.UnlessInsideWorktree {
			return w.cx.outside(s, operandsOf(s, name, w.cx.fl))
		}
		return true
	}
	if excluded(rule, name, s, env) {
		return false
	}
	return regexMatches(w, rule, s.text, s)
}

func excluded(rule *Rule, name string, s *Simple, env *matchEnv) bool {
	for _, ex := range rule.c.excludes {
		if ex.match(name, s.Argv[1:], env) {
			return true
		}
	}
	return false
}

// regexMatches applies a regex rule to text.
func regexMatches(w *walker, rule *Rule, text string, s *Simple) bool {
	for _, re := range rule.c.res {
		if !re.MatchString(text) {
			return false
		}
	}
	if rule.UnlessInsideWorktree {
		for _, m := range rule.c.res[0].FindAllStringSubmatch(text, -1) {
			if rule.c.pathGroup < len(m) && w.cx.pathOutside(m[rule.c.pathGroup], s) {
				return true
			}
		}
		return false
	}
	return true
}
