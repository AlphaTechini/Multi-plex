package gate

import (
	"fmt"
	"strings"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/safety/pathpolicy"
)

// evalFiles checks the paths of a file tool: writes (Layer 2, plus protected
// settings) or reads (Layer 3 deny-read).
func (g *Gate) evalFiles(ev agent.HookEvent, write bool) Verdict {
	var bad []pathpolicy.Verdict
	var shown []string
	for _, p := range ev.Paths {
		if p == "" {
			continue
		}
		p = g.fromCwd(ev.Cwd, p)
		var v pathpolicy.Verdict
		if write {
			v = g.Policy.CheckWrite(p)
		} else {
			v = g.Policy.CheckRead(p)
		}
		if !v.Allowed {
			bad = append(bad, v)
			shown = append(shown, g.display(v))
		}
	}
	if len(bad) == 0 {
		return Verdict{Decision: DecisionAllow}
	}
	first := bad[0]
	out := Verdict{
		Decision: DecisionGate, Kind: core.ApprovalFileRead, Rule: first.Rule, Layer: layerOf(first.Rule),
		Reason: reasonFor(first, shown[0], len(bad)), Paths: shown, Cwd: g.cwdDisplay(ev.Cwd),
	}
	if write {
		out.Kind = core.ApprovalFileWrite
	}
	return out
}

// layerOf maps a path-policy rule to the layer that owns it.
func layerOf(rule string) string {
	if rule == pathpolicy.RuleDenyRead {
		return LayerCredentials
	}
	return LayerWrites
}

func reasonFor(v pathpolicy.Verdict, shown string, n int) string {
	r := fmt.Sprintf("%s: %s", shown, v.Reason)
	if n > 1 {
		r += fmt.Sprintf(" (and %d more)", n-1)
	}
	return r
}

// display returns the path of a verdict relative to the worktree where
// possible.
func (g *Gate) display(v pathpolicy.Verdict) string { return g.Policy.Display(v.Path) }

func (g *Gate) cwdDisplay(cwd string) string {
	if cwd == "" {
		return ""
	}
	return g.Policy.Display(cwd)
}

// fromCwd makes a relative tool path relative to the call's working
// directory, which is how the tool itself resolves it. "~" paths and
// absolute paths are returned unchanged.
func (g *Gate) fromCwd(cwd, p string) string {
	if cwd == "" || strings.HasPrefix(p, "~") {
		return p
	}
	goos := g.Policy.OS()
	if pathpolicy.IsAbsOS(goos, p) {
		return p
	}
	return cwd + "/" + p
}
