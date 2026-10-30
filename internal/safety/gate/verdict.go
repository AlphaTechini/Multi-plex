// Package gate combines the safety layers for one pre-tool hook event
// (docs/04-safety.md): Layer 2 (writes outside the worktree), Layer 3
// (credential stores) and Layer 4 (command classification). It decides
// whether the action may proceed silently, needs a human-present approval
// (Layer 5), or is refused; it does not prompt or wait itself.
package gate

import "github.com/alphatechini/multi-plex/internal/core"

// Decision is the gate's answer for one tool call.
type Decision string

// Decisions.
const (
	DecisionAllow Decision = "allow" // proceed, no prompt
	DecisionGate  Decision = "gate"  // ask a human (Layer 5) before proceeding
	DecisionDeny  Decision = "deny"  // refuse without asking (malformed input)
)

// Layers named in a Verdict and in approval requests.
const (
	LayerWrites      = "layer-2-writes"
	LayerCredentials = "layer-3-credentials"
	LayerCommand     = "layer-4-command"
)

// Verdict is the result of evaluating one hook event.
type Verdict struct {
	Decision Decision
	Kind     core.ApprovalKind // what kind of approval to ask for
	Rule     string            // rule id that fired
	Layer    string            // one of the Layer constants
	Reason   string            // why it was gated, for the prompt
	Paths    []string          // affected paths, relative to the worktree where possible, slash-separated
	Cwd      string            // working directory for display, relative to the worktree where possible
}

// Allowed reports whether the call may proceed without approval.
func (v Verdict) Allowed() bool { return v.Decision == DecisionAllow }
