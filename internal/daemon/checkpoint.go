package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// checkpointSkipWindow is how recent a checkpoint must be, with no write since,
// for the next shell command to reuse it instead of taking another.
const checkpointSkipWindow = 2 * time.Second

// checkpointRef is the ref a checkpoint is stored under (docs/04 Layer 1).
func checkpointRef(id core.SessionID, n int) string {
	return fmt.Sprintf("refs/multiplex/checkpoints/%s/%d", id, n)
}

// takeCheckpoint snapshots the session's worktree under the next number and
// records it in the store.
func (d *Daemon) takeCheckpoint(ctx context.Context, s *session, reason string) (core.Checkpoint, error) {
	s.ckptMu.Lock()
	defer s.ckptMu.Unlock()
	return d.takeCheckpointLocked(ctx, s, reason)
}

// takeCheckpointLocked is takeCheckpoint for callers holding s.ckptMu.
func (d *Daemon) takeCheckpointLocked(ctx context.Context, s *session, reason string) (core.Checkpoint, error) {
	if !s.ckptKnown {
		list, err := d.store.ListCheckpoints(ctx, s.id)
		if err != nil {
			return core.Checkpoint{}, err
		}
		for _, c := range list {
			s.nextCkpt = max(s.nextCkpt, c.N)
		}
		s.ckptKnown = true
	}
	n := s.nextCkpt + 1
	ref := checkpointRef(s.id, n)
	info := s.snap()
	commit, err := d.vcs.Checkpoint(ctx, info.WorktreePath, ref, "multiplex checkpoint: "+reason)
	if err != nil {
		return core.Checkpoint{}, err
	}
	cp := core.Checkpoint{SessionID: s.id, N: n, Ref: ref, Commit: commit, Reason: reason, At: d.now()}
	if err := d.store.AddCheckpoint(ctx, cp); err != nil {
		return core.Checkpoint{}, err
	}
	s.nextCkpt = n
	s.mu.Lock()
	s.lastCkpt, s.dirty = time.Now(), false
	s.mu.Unlock()
	return cp, nil
}

// checkpointBeforeShell takes the Layer 1 checkpoint before a shell command,
// unless the previous one is under two seconds old and nothing was written
// since. A failure is logged, not fatal: the gate and the other layers still
// apply.
func (d *Daemon) checkpointBeforeShell(ctx context.Context, s *session) {
	s.mu.Lock()
	recent := !s.lastCkpt.IsZero() && time.Since(s.lastCkpt) < checkpointSkipWindow && !s.dirty
	s.mu.Unlock()
	if recent {
		return
	}
	if _, err := d.takeCheckpoint(ctx, s, "before shell command"); err != nil {
		d.log.Warn("checkpoint before shell command failed", "session", s.id, "err", err)
	}
}

// hCheckpointList lists a session's checkpoints.
func (d *Daemon) hCheckpointList(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	if _, err := d.getSession(p.SessionID); err != nil {
		return nil, err
	}
	list, err := d.store.ListCheckpoints(ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []core.Checkpoint{}
	}
	return list, nil
}

// hCheckpointRestore rolls a session's worktree back to checkpoint N. A new
// checkpoint "before restore" is taken first, so the restore itself can be
// undone.
func (d *Daemon) hCheckpointRestore(ctx context.Context, _ *conn, p protocol.CheckpointRestoreParams) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	list, err := d.store.ListCheckpoints(ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	var target *core.Checkpoint
	for i := range list {
		if list[i].N == p.N {
			target = &list[i]
		}
	}
	if target == nil {
		return nil, notFound("session has no checkpoint %d", p.N)
	}

	s.ckptMu.Lock()
	defer s.ckptMu.Unlock()
	if _, err := d.takeCheckpointLocked(ctx, s, "before restore"); err != nil {
		return nil, fmt.Errorf("cannot take the safety checkpoint before restoring: %w", err)
	}
	if err := d.vcs.Restore(ctx, s.snap().WorktreePath, target.Commit); err != nil {
		return nil, err
	}
	d.announceDiff(s)
	return protocol.Empty{}, nil
}
