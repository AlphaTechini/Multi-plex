package gate

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/safety/classify"
	"github.com/alphatechini/multi-plex/internal/safety/pathpolicy"
)

// evalShell runs a shell command through Layer 4 (classification), then
// scans it for credential stores (Layer 3) and writes outside the worktree
// (Layer 2). The first layer to object decides the verdict.
func (g *Gate) evalShell(ev agent.HookEvent) Verdict {
	rules := g.Rules
	if rules == nil {
		rules = classify.Default()
	}
	ctx := classify.Context{
		WorktreeRoot:      g.Policy.WorktreeRoot,
		Cwd:               ev.Cwd,
		Shell:             g.Shell,
		ProtectedBranches: g.ProtectedBranches,
		CurrentBranch:     g.CurrentBranch,
		Home:              g.Policy.Home,
		SafeDirs:          g.Policy.WritableDirs(),
	}
	an := rules.Analyze(ev.Command, ctx, true)
	cwd := g.cwdDisplay(ev.Cwd)
	if an.Result.Class == classify.Dangerous {
		kind := core.ApprovalCommand
		if an.Result.Kind == "push" {
			kind = core.ApprovalPush
		}
		return Verdict{Decision: DecisionGate, Kind: kind, Rule: an.Result.RuleID, Layer: LayerCommand, Reason: an.Result.Reason, Cwd: cwd}
	}
	if v, ok := g.scanCredentials(ev, an); ok {
		v.Cwd = cwd
		return v
	}
	if v, ok := g.scanWrites(an); ok {
		v.Cwd = cwd
		return v
	}
	if v, ok := g.scanScripts(an, rules, ctx); ok {
		v.Cwd = cwd
		return v
	}
	return Verdict{Decision: DecisionAllow}
}

// scanCredentials gates commands that name a credential store, either as a
// literal anywhere in the text or as a path argument or redirection.
func (g *Gate) scanCredentials(ev agent.HookEvent, an classify.Analysis) (Verdict, bool) {
	pol := g.Policy.WithoutSymlinks()
	deny := func(path string) (Verdict, bool) {
		return Verdict{
			Decision: DecisionGate, Kind: core.ApprovalCommand, Rule: pathpolicy.RuleDenyRead, Layer: LayerCredentials,
			Reason: "the command touches a credential store: " + pol.Display(path), Paths: []string{pol.Display(path)},
		}, true
	}
	if entry, ok := pol.MentionsDenied(ev.Command); ok {
		return deny(entry)
	}
	for i := range an.Commands {
		s := &an.Commands[i]
		toks := make([]string, 0, len(s.Argv)+len(s.Redirects))
		toks = append(toks, s.Argv[1:]...)
		for _, r := range s.Redirects {
			toks = append(toks, r.Target)
		}
		for _, t := range toks {
			for _, cand := range pathCandidates(t) {
				abs, ok := an.Resolver.Resolve(cand, s.Cwd, s.CwdKnown)
				if !ok {
					continue
				}
				if v := pol.CheckRead(abs); !v.Allowed && v.Rule == pathpolicy.RuleDenyRead {
					return deny(v.Path)
				}
			}
		}
	}
	return Verdict{}, false
}

// pathCandidates returns the path-like parts of one argument: the argument
// itself and, for NAME=value or --opt=value, the value.
func pathCandidates(tok string) []string {
	if tok == "" {
		return nil
	}
	if tok[0] == '-' && !strings.Contains(tok, "=") {
		return nil
	}
	if i := strings.IndexByte(tok, '='); i > 0 && i < len(tok)-1 {
		return []string{tok, tok[i+1:]}
	}
	return []string{tok}
}

var (
	writeLast = map[string]bool{"cp": true, "mv": true, "install": true, "ln": true, "copy-item": true, "move-item": true,
		"cpi": true, "mi": true, "copy": true, "move": true, "xcopy": true}
	writeAll = map[string]bool{"tee": true, "touch": true, "mkdir": true, "md": true, "truncate": true, "set-content": true,
		"add-content": true, "out-file": true, "new-item": true, "ni": true, "tee-object": true, "mkfifo": true}
	deleteCmds = map[string]bool{"rm": true, "unlink": true, "rmdir": true, "del": true, "erase": true, "rd": true,
		"ri": true, "remove-item": true, "shred": true}
)

// scanWrites applies Layer 2 to what a shell command writes: redirection
// targets and the destinations of common write commands. Deletes are only
// checked against protected paths, since Layer 4 already handles deletes
// outside the worktree.
func (g *Gate) scanWrites(an classify.Analysis) (Verdict, bool) {
	pol := g.Policy
	check := func(s *classify.Simple, tok string, onlyProtected bool) (Verdict, bool) {
		abs, ok := an.Resolver.Resolve(tok, s.Cwd, s.CwdKnown)
		if !ok {
			return Verdict{}, false
		}
		v := pol.CheckWrite(abs)
		if v.Allowed {
			return Verdict{}, false
		}
		if onlyProtected && v.Rule == pathpolicy.RuleOutsideWorktree {
			return Verdict{}, false
		}
		shown := pol.Display(v.Path)
		return Verdict{
			Decision: DecisionGate, Kind: core.ApprovalCommand, Rule: v.Rule, Layer: layerOf(v.Rule),
			Reason: "the command writes " + shown + ": " + v.Reason, Paths: []string{shown},
		}, true
	}
	for i := range an.Commands {
		s := &an.Commands[i]
		for _, r := range s.Redirects {
			if r.IsOutput() && !r.IsSink() {
				if v, ok := check(s, r.Target, false); ok {
					return v, true
				}
			}
		}
		name := s.Name()
		switch {
		case writeLast[name]:
			if ops := s.Operands(); len(ops) > 0 {
				if v, ok := check(s, ops[len(ops)-1], false); ok {
					return v, true
				}
			}
		case writeAll[name]:
			for _, op := range s.Operands() {
				if (classify.Redirect{Target: op}).IsSink() {
					continue
				}
				if v, ok := check(s, op, false); ok {
					return v, true
				}
			}
		case deleteCmds[name]:
			for _, op := range s.Operands() {
				if v, ok := check(s, op, true); ok {
					return v, true
				}
			}
		}
	}
	return Verdict{}, false
}
