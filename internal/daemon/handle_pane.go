package daemon

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// hPaneOpen opens a shell pane in a session's worktree.
func (d *Daemon) hPaneOpen(ctx context.Context, _ *conn, p protocol.PaneOpenParams) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	sh, ok := agent.Get("shell")
	if !ok {
		return nil, badRequest("the shell adapter is not available")
	}
	info := s.snap()
	ws, err := d.workspace(ctx, info.WorkspaceID)
	if err != nil {
		return nil, err
	}
	base := d.ports.Alloc(info.ID)
	launch, err := sh.Prepare(agent.LaunchSpec{
		Session:       info,
		WorkspaceRoot: ws.Root,
		WorktreePath:  info.WorktreePath,
		ConfigDir:     d.sessionConfigDir(info.ID),
		MpxPath:       d.mpxPath,
		HookCommand:   d.hookCmd,
		Env:           d.sessionEnv(info.ID, base),
	})
	if err != nil {
		return nil, badRequest("cannot start a shell: %v", err)
	}
	pn, err := d.startPane(s, core.PaneShell, launch, p.Cols, p.Rows, nil)
	if err != nil {
		return nil, badRequest("cannot start a shell: %v", err)
	}
	return pn.snapshotInfo(), nil
}

// hPaneClose closes a pane. Closing a session's agent pane stops the agent.
func (d *Daemon) hPaneClose(_ context.Context, _ *conn, p protocol.PaneRef) (any, error) {
	pn, err := d.paneByID(p.PaneID)
	if err != nil {
		return nil, err
	}
	if pn.kind == core.PaneAgent {
		d.stopSession(pn.sess, "closed")
		return protocol.Empty{}, nil
	}
	d.removePane(pn)
	return protocol.Empty{}, nil
}

// hPaneInput queues keystrokes for a pane. It runs on the connection's read
// loop so input keeps its order.
func (d *Daemon) hPaneInput(_ context.Context, _ *conn, p protocol.PaneInputParams) (any, error) {
	pn, err := d.paneByID(p.PaneID)
	if err != nil {
		return nil, err
	}
	if err := pn.write(p.Data); err != nil {
		return nil, badRequest("%v", err)
	}
	return protocol.Empty{}, nil
}

// hPaneResize resizes a pane's terminal.
func (d *Daemon) hPaneResize(_ context.Context, _ *conn, p protocol.PaneResizeParams) (any, error) {
	pn, err := d.paneByID(p.PaneID)
	if err != nil {
		return nil, err
	}
	h := pn.host.Load()
	if h == nil {
		return nil, badRequest("pane %s is not running", p.PaneID)
	}
	if err := h.Resize(p.Cols, p.Rows); err != nil {
		return nil, badRequest("%v", err)
	}
	pn.mu.Lock()
	pn.info.Cols, pn.info.Rows = p.Cols, p.Rows
	pn.mu.Unlock()
	return protocol.Empty{}, nil
}

// hPaneSnapshot returns the full screen of a pane.
func (d *Daemon) hPaneSnapshot(_ context.Context, _ *conn, p protocol.PaneRef) (any, error) {
	pn, err := d.paneByID(p.PaneID)
	if err != nil {
		return nil, err
	}
	h := pn.host.Load()
	if h == nil {
		return nil, badRequest("pane %s is not running", p.PaneID)
	}
	return h.Snapshot(), nil
}
