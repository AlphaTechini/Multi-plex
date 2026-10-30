package gate

import (
	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/safety/classify"
	"github.com/alphatechini/multi-plex/internal/safety/pathpolicy"
)

// Gate evaluates pre-tool hook events against the safety layers. Fill the
// fields once per session; Evaluate is safe for concurrent use.
type Gate struct {
	// Rules are the Layer 4 rules; nil means classify.Default().
	Rules *classify.Rules
	// Policy is the Layer 2 and 3 path policy of the session's worktree.
	Policy pathpolicy.Policy
	// ProtectedBranches are the shared branches; empty means main and master.
	ProtectedBranches []string
	// CurrentBranch is the worktree's branch, if known (see classify.Context).
	CurrentBranch string
	// Shell is "bash", "powershell", "cmd" or "" for the shell agent commands
	// run in.
	Shell string
}

// Evaluate decides one hook event. Only pre-tool events are gated; others
// are allowed. Unknown commands and tools are allowed: they rely on
// Layers 1 to 3 (docs/04).
func (g *Gate) Evaluate(ev agent.HookEvent) Verdict {
	if ev.Kind != agent.HookPreTool && ev.Kind != "" {
		return Verdict{Decision: DecisionAllow}
	}
	switch ev.ToolClass {
	case agent.ToolShell:
		return g.evalShell(ev)
	case agent.ToolWrite:
		return g.evalFiles(ev, true)
	case agent.ToolRead:
		return g.evalFiles(ev, false)
	}
	if ev.ToolClass == "" && ev.Command != "" {
		return g.evalShell(ev)
	}
	return Verdict{Decision: DecisionAllow}
}
