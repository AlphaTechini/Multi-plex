package daemon

import (
	"context"
	"time"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// removeTimeout bounds the worktree removal, which retries on Windows.
const removeTimeout = 60 * time.Second

// hSessionRemove stops a session, removes its worktree and branch, and
// deletes its records. Checkpoint refs are kept (docs/05 "Clean up").
func (d *Daemon) hSessionRemove(_ context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	info := s.snap()
	d.stopSession(s, "removing")
	for _, pn := range d.panesOf(s) {
		d.removePane(pn) // shells too: open files block removal on Windows
	}

	ctx, cancel := context.WithTimeout(d.ctx, removeTimeout)
	defer cancel()
	if ws, werr := d.workspace(ctx, info.WorkspaceID); werr == nil {
		if err := d.vcs.WorktreeRemove(ctx, ws.Root, info.WorktreePath); err != nil {
			return nil, badRequest("cannot remove the worktree: %v", err)
		}
		if err := d.vcs.DeleteBranch(ctx, ws.Root, info.Branch); err != nil {
			d.log.Warn("delete branch", "branch", info.Branch, "err", err)
		}
	}

	s.mu.Lock()
	if s.diffTimer != nil {
		s.diffTimer.Stop()
	}
	s.mu.Unlock()

	d.mu.Lock()
	delete(d.sessions, s.id)
	d.mu.Unlock()
	d.rate.Forget(s.id)
	d.ports.Release(s.id)
	s.pmu.Lock() // let any in-flight save finish before the delete
	err = d.store.DeleteSession(d.ctx, s.id)
	s.pmu.Unlock()
	if err != nil {
		d.log.Error("delete session", "session", s.id, "err", err)
	}
	d.publish(protocol.TopicSessionRemoved, protocol.SessionRef{SessionID: s.id})
	d.notifyState()
	return protocol.Empty{}, nil
}
