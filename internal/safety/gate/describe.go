package gate

import (
	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
)

// Describe builds the approval request for a gated call, with what the
// prompt must show (docs/04, Layer 5): the exact command or file operation,
// the working directory, which rule and layer gated it and which files are
// affected. Paths are relative to the worktree where possible.
//
// The caller fills in the fields only it knows: ID, SessionID and At.
func Describe(v Verdict, ev agent.HookEvent) core.ApprovalRequest {
	kind := v.Kind
	if kind == "" {
		switch ev.ToolClass {
		case agent.ToolWrite:
			kind = core.ApprovalFileWrite
		case agent.ToolRead:
			kind = core.ApprovalFileRead
		default:
			kind = core.ApprovalCommand
		}
	}
	cwd := v.Cwd
	if cwd == "" {
		cwd = ev.Cwd
	}
	return core.ApprovalRequest{
		Kind:    kind,
		Tool:    ev.ToolName,
		Command: ev.Command,
		Cwd:     cwd,
		Paths:   append([]string(nil), v.Paths...),
		Rule:    v.Rule,
		Layer:   v.Layer,
	}
}
