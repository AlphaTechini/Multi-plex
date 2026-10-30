package daemon

import (
	"context"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/safety/gate"
)

// Push broker settings (docs/04-safety.md Layer 3).
const (
	pushRemote  = "origin"
	pushRule    = "push-broker"
	pushTimeout = 5 * time.Minute
)

// hSessionPush is the push broker behind `mpx push`: it asks for a
// human-present approval and, only if that is granted, pushes the session's
// branch from the main repository (not the worktree, whose push URL is
// disabled) with the user's own environment and credentials.
func (d *Daemon) hSessionPush(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	if d.remotes == nil {
		return nil, badRequest("pushing is not available in this daemon")
	}
	info := s.snap()
	ws, err := d.workspace(ctx, info.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if !d.remotes.HasRemote(ctx, ws.Root, pushRemote) {
		return nil, badRequest("the repository has no %q remote to push to", pushRemote)
	}

	dec, err := d.requestApproval(ctx, s, core.ApprovalRequest{
		Kind:    core.ApprovalPush,
		Tool:    "mpx push",
		Command: "git push " + pushRemote + " " + info.Branch,
		Cwd:     ".",
		Rule:    pushRule,
		Layer:   gate.LayerCredentials,
	})
	if err != nil && dec.Outcome == "" {
		return nil, denied("the push approval could not be requested: %v", err)
	}
	if dec.Outcome != core.Approved {
		return nil, denied("%s", denyReason(dec))
	}

	pctx, cancel := context.WithTimeout(d.ctx, pushTimeout)
	defer cancel()
	out, err := d.remotes.Push(pctx, ws.Root, pushRemote, info.Branch)
	if err != nil {
		d.log.Warn("push failed", "session", info.ID, "branch", info.Branch)
		return nil, badRequest("git push failed: %v\n%s", err, out)
	}
	d.log.Info("branch pushed", "session", info.ID, "branch", info.Branch)
	return protocol.PushResult{Remote: pushRemote, Branch: info.Branch, Output: out}, nil
}
