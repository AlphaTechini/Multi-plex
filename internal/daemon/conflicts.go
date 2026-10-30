package daemon

import (
	"context"
	"reflect"
	"slices"
	"strings"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// predictConflicts tests the session's work against its base branch and every
// other session of the workspace (docs/05 "Conflicts"). It looks at what is
// in each worktree now, committed or not: a snapshot commit of the worktree is
// merged in memory with git merge-tree, so no file is touched.
func (d *Daemon) predictConflicts(ctx context.Context, s *session) {
	info := s.snap()
	ws, err := d.workspace(ctx, info.WorkspaceID)
	if err != nil {
		return
	}
	mine := d.snapshotTip(ctx, s)

	var found []core.Conflict
	files, err := d.vcs.MergeConflicts(ctx, ws.Root, mine, info.BaseBranch)
	switch {
	case err != nil:
		d.log.Debug("conflict prediction against base", "session", info.ID, "err", err)
	case len(files) > 0:
		found = append(found, core.Conflict{SessionID: info.ID, OtherBranch: info.BaseBranch, Files: files})
	}

	for _, o := range d.sessionsOf(info.WorkspaceID) {
		if o == s {
			continue
		}
		oi := o.snap()
		files, err := d.vcs.MergeConflicts(ctx, ws.Root, mine, d.snapshotTip(ctx, o))
		if err != nil {
			d.log.Debug("conflict prediction", "session", info.ID, "other", oi.ID, "err", err)
			continue
		}
		if len(files) > 0 {
			found = append(found, core.Conflict{SessionID: info.ID, OtherBranch: oi.Branch, OtherSession: oi.ID, Files: files})
		}
		// The same conflict, seen from the other session.
		d.setConflictWith(o, info, files)
	}
	d.storeConflicts(s, found)
}

// snapshotTip returns a commit holding the session's worktree as it is now,
// or its branch name if a snapshot cannot be taken.
func (d *Daemon) snapshotTip(ctx context.Context, s *session) string {
	info := s.snap()
	commit, err := d.vcs.Checkpoint(ctx, info.WorktreePath, "refs/multiplex/predict/"+string(info.ID), "multiplex conflict prediction")
	if err != nil {
		return info.Branch
	}
	return commit
}

// setConflictWith updates o's conflict list for its pairing with other.
func (d *Daemon) setConflictWith(o *session, other core.Session, files []string) {
	o.mu.Lock()
	list := slices.DeleteFunc(slices.Clone(o.conflicts), func(c core.Conflict) bool { return c.OtherSession == other.ID })
	o.mu.Unlock()
	if len(files) > 0 {
		list = append(list, core.Conflict{SessionID: o.id, OtherBranch: other.Branch, OtherSession: other.ID, Files: files})
	}
	d.storeConflicts(o, list)
}

// storeConflicts keeps a session's conflict list and announces a change.
func (d *Daemon) storeConflicts(s *session, list []core.Conflict) {
	slices.SortFunc(list, func(a, b core.Conflict) int { return strings.Compare(a.OtherBranch, b.OtherBranch) })
	s.mu.Lock()
	changed := !reflect.DeepEqual(nilIfEmpty(s.conflicts), nilIfEmpty(list))
	if changed {
		s.conflicts = list
	}
	s.mu.Unlock()
	if changed {
		if list == nil {
			list = []core.Conflict{}
		}
		d.publish(protocol.TopicConflicts, protocol.ConflictsEvent{SessionID: s.id, Conflicts: list})
	}
}

func nilIfEmpty(l []core.Conflict) []core.Conflict {
	if len(l) == 0 {
		return nil
	}
	return l
}
