package daemon

import (
	"context"
	"os"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// hSessionRestart relaunches the agent in the session's existing worktree.
func (d *Daemon) hSessionRestart(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	info := s.snap()
	if _, err := os.Stat(info.WorktreePath); err != nil {
		return nil, badRequest("the worktree %s is gone; remove the session and create a new one", info.WorktreePath)
	}
	ws, err := d.workspace(ctx, info.WorkspaceID)
	if err != nil {
		return nil, err
	}
	d.stopSession(s, "restarting")
	d.launchAgent(d.ctx, s, ws, 0, 0, false)
	return s.snap(), nil
}
