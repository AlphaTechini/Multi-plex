package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/safety/gate"
)

// hookPreTool is the gate (docs/04-safety.md, wrapper mode enforcement). The
// hot path, an allowed call, does no disk I/O except the checkpoint taken
// before a shell command.
func (d *Daemon) hookPreTool(ctx context.Context, s *session, ev agent.HookEvent) agent.HookResponse {
	d.markWorking(s)
	if isShell(ev) {
		d.checkpointBeforeShell(ctx, s)
	}
	v := d.gateFor(s).Evaluate(ev)
	switch v.Decision {
	case gate.DecisionAllow:
		return agent.HookResponse{Decision: agent.Allow}
	case gate.DecisionDeny:
		return agent.HookResponse{Decision: agent.Deny, Reason: "Blocked by Multiplex: " + v.Reason}
	}

	dec, err := d.requestApproval(ctx, s, gate.Describe(v, ev))
	if err != nil && dec.Outcome == "" {
		d.log.Warn("approval request failed", "session", s.id, "err", err)
		reason := "approval could not be requested"
		if errors.Is(err, context.Canceled) {
			reason = "the approval request was cancelled"
		}
		return agent.HookResponse{Decision: agent.Deny, Reason: "Blocked by Multiplex: " + reason}
	}
	if dec.Outcome == core.Approved {
		return agent.HookResponse{Decision: agent.Allow}
	}
	return agent.HookResponse{Decision: agent.Deny, Reason: denyReason(dec)}
}

// denyReason is the text the agent sees when an approval is denied. It
// includes the message the user typed.
func denyReason(dec core.ApprovalDecision) string {
	switch dec.Message {
	case "":
		return "Denied by the user."
	case "approval timed out", "approval request cancelled", "no approval method available", "approval verification failed":
		return "Blocked by Multiplex: " + dec.Message + "."
	}
	return "Denied by the user: " + dec.Message
}

// relativeTo returns p relative to root as a slash path, or p slash-separated
// when it is not under root.
func relativeTo(root, p string) string {
	if !filepath.IsAbs(p) {
		return filepath.ToSlash(filepath.Clean(p))
	}
	if rel, ok := within(root, p); ok {
		return rel
	}
	r1, e1 := filepath.EvalSymlinks(root)
	p1, e2 := filepath.EvalSymlinks(p)
	if e1 == nil && e2 == nil {
		if rel, ok := within(r1, p1); ok {
			return rel
		}
	}
	return filepath.ToSlash(p)
}

func within(root, p string) (string, bool) {
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
