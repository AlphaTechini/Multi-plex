package daemon

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// hSessionStop kills the agent and keeps the worktree.
func (d *Daemon) hSessionStop(_ context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	d.stopSession(s, "stopped")
	return protocol.Empty{}, nil
}

// stopSession kills the session's agent pane and moves the session to idle.
// Shell panes stay open. The detail says why.
func (d *Daemon) stopSession(s *session, detail string) {
	s.mu.Lock()
	s.stopping = true
	paneID := s.info.AgentPane
	s.mu.Unlock()

	if paneID != "" {
		if p, err := d.paneByID(paneID); err == nil {
			d.removePane(p)
		}
	}
	d.ports.Release(s.id)
	d.update(s, func(c *core.Session) {
		c.AgentPane = ""
		if c.State.Active() || c.State == core.StateDone {
			c.State = core.StateIdle
		}
		c.StateDetail = detail
	})
}
