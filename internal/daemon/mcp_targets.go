package daemon

import (
	"context"
	"strings"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

// resolveSessions finds the sessions named by refs (an ID or a slug) among
// the caller's workspace. An agent can only see its own workspace. With no
// refs it returns every other session there.
func (d *Daemon) resolveSessions(caller *session, refs []string) ([]*session, error) {
	ws := caller.snap().WorkspaceID
	all := d.sessionsOf(ws)
	if len(refs) == 0 {
		out := make([]*session, 0, len(all))
		for _, s := range all {
			if s != caller {
				out = append(out, s)
			}
		}
		return out, nil
	}
	var out []*session
	seen := map[core.SessionID]bool{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		var match *session
		for _, s := range all {
			if i := s.snap(); string(i.ID) == ref || i.Slug == ref {
				match = s
				break
			}
		}
		if match == nil {
			return nil, notFound("no session %q in this workspace", ref)
		}
		if !seen[match.id] {
			seen[match.id] = true
			out = append(out, match)
		}
	}
	return out, nil
}

// sessionBrief is one session in a status result.
type sessionBrief struct {
	ID          core.SessionID    `json:"id"`
	Slug        string            `json:"slug"`
	Task        string            `json:"task"`
	Role        string            `json:"role"`
	Branch      string            `json:"branch"`
	State       core.SessionState `json:"state"`
	StateDetail string            `json:"state_detail,omitempty"`
	Summary     *core.Summary     `json:"summary,omitempty"`
	Notes       []noteBrief       `json:"notes,omitempty"`
}

// noteBrief is a card in a status result.
type noteBrief struct {
	Number   int           `json:"number"`
	Title    string        `json:"title"`
	Severity core.Severity `json:"severity"`
	Body     string        `json:"body,omitempty"`
}

// briefOf builds a session's status entry: state, latest summary and the
// cards that have not been dismissed.
func (d *Daemon) briefOf(ctx context.Context, s *session) sessionBrief {
	info := s.snap()
	b := sessionBrief{
		ID: info.ID, Slug: info.Slug, Task: info.Task, Role: info.Role,
		Branch: info.Branch, State: info.State, StateDetail: info.StateDetail,
	}
	if sum, err := d.store.LatestSummary(ctx, info.ID); err == nil {
		b.Summary = &sum
	} else if err != store.ErrNotFound {
		d.log.Debug("status summary", "session", info.ID, "err", err)
	}
	if notes, err := d.store.ListNotes(ctx, info.ID); err == nil {
		for _, n := range notes {
			if !n.Dismissed {
				b.Notes = append(b.Notes, noteBrief{Number: n.Number, Title: n.Title, Severity: n.Severity, Body: n.Body})
			}
		}
	}
	return b
}
