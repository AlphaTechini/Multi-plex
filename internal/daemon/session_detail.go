package daemon

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/store"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// changesFresh is how long the cached change list of a session is trusted.
const changesFresh = 2 * time.Second

// lastMessageLines is how many screen lines make the Overview fallback text.
const lastMessageLines = 8

// hSessionList lists sessions, oldest first.
func (d *Daemon) hSessionList(_ context.Context, _ *conn, p protocol.SessionListParams) (any, error) {
	list := d.sessionsOf(p.WorkspaceID)
	out := make([]core.Session, len(list))
	for i, s := range list {
		out[i] = s.snap()
	}
	return out, nil
}

// hSessionGet assembles everything the Overview needs for one session.
func (d *Daemon) hSessionGet(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	return d.sessionDetail(ctx, s)
}

// sessionDetail builds the SessionDetail of a session.
func (d *Daemon) sessionDetail(ctx context.Context, s *session) (protocol.SessionDetail, error) {
	info := s.snap()
	det := protocol.SessionDetail{
		Session:   info,
		Notes:     []core.Note{},
		Conflicts: []core.Conflict{},
		Changes:   []vcs.FileChange{},
		Approvals: d.rate.PerHour(info.ID, d.now()),
	}

	sum, err := d.store.LatestSummary(ctx, info.ID)
	switch {
	case err == nil:
		det.Summary = &sum
	case !errors.Is(err, store.ErrNotFound):
		return det, err
	}
	if notes, err := d.store.ListNotes(ctx, info.ID); err != nil {
		return det, err
	} else if notes != nil {
		det.Notes = notes
	}
	if cps, err := d.store.ListCheckpoints(ctx, info.ID); err != nil {
		return det, err
	} else {
		det.Checkpoints = len(cps)
	}

	if sums, err := d.store.ListSummaries(ctx, info.ID, 0); err != nil {
		return det, err
	} else {
		det.Reports = len(sums)
	}

	s.mu.Lock()
	det.Conflicts = slices.Clone(s.conflicts)
	det.Turn = s.turn
	s.mu.Unlock()
	if det.Conflicts == nil {
		det.Conflicts = []core.Conflict{}
	}

	if changes, err := d.changesOf(ctx, s); err != nil {
		d.log.Debug("changed files", "session", info.ID, "err", err)
	} else if changes != nil {
		det.Changes = changes
	}

	if det.Summary == nil {
		det.LastMessage = d.lastMessage(s)
	}
	return det, nil
}

// lastMessage returns the last lines of the agent's screen, the Overview's
// fallback when the agent never called report_progress (docs/08).
func (d *Daemon) lastMessage(s *session) string {
	p, err := d.agentPane(s)
	if err != nil {
		return ""
	}
	h := p.host.Load()
	if h == nil {
		return ""
	}
	return strings.Join(h.LastLines(lastMessageLines), "\n")
}
