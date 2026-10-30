package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/safety/gate"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// session is the daemon's live record of one session: the persisted
// core.Session plus the runtime state that does not survive a restart.
type session struct {
	id core.SessionID

	// pmu orders "save and publish" so the store and the event stream always
	// end on the latest state, whichever goroutine changed it last.
	pmu sync.Mutex

	mu   sync.Mutex // guards everything below
	info core.Session

	stopping   bool // a stop was requested; the agent's exit is expected
	gate       *gate.Gate
	gateWhen   time.Time
	blockCount int // approvals currently waiting

	// Checkpoint bookkeeping (docs/04 Layer 1).
	ckptMu    sync.Mutex
	nextCkpt  int
	ckptKnown bool
	lastCkpt  time.Time
	dirty     bool // a write happened since the last checkpoint

	// Task typing: the task is typed into a hook-capable agent when its
	// SessionStart hook arrives, once per session.
	taskPending bool
	taskTyped   bool

	conflicts []core.Conflict
	changes   []vcs.FileChange
	changesAt time.Time
	diffSig   string
	diffTimer *time.Timer
	lastEdit  time.Time
}

func newSession(info core.Session) *session {
	return &session{id: info.ID, info: info}
}

// snap returns a copy of the persisted record.
func (s *session) snap() core.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info
}

// getSession finds a live session.
func (d *Daemon) getSession(id core.SessionID) (*session, error) {
	d.mu.RLock()
	s := d.sessions[id]
	d.mu.RUnlock()
	if s == nil {
		return nil, notFound("no such session %q", id)
	}
	return s, nil
}

// update applies fn to the session record. If anything changed it bumps
// UpdatedAt, saves the record, publishes session.updated and wakes
// session_wait callers. It returns the record as it is afterwards.
func (d *Daemon) update(s *session, fn func(*core.Session)) core.Session {
	s.mu.Lock()
	before := s.info
	fn(&s.info)
	changed := before != s.info
	if changed {
		s.info.UpdatedAt = d.now()
	}
	cur := s.info
	s.mu.Unlock()
	if changed {
		d.persistAndPublish(s)
	}
	return cur
}

// persistAndPublish saves the session's current record and announces it.
func (d *Daemon) persistAndPublish(s *session) {
	s.pmu.Lock()
	cur := s.snap()
	if err := d.store.PutSession(d.ctx, cur); err != nil {
		d.log.Error("save session", "session", cur.ID, "err", err)
	}
	d.publish(protocol.TopicSession, cur)
	s.pmu.Unlock()
	d.notifyState()
}

// setState moves the session to state with a detail line.
func (d *Daemon) setState(s *session, state core.SessionState, detail string) core.Session {
	return d.update(s, func(c *core.Session) {
		c.State, c.StateDetail = state, detail
	})
}

// notifyState wakes everything waiting for a session change.
func (d *Daemon) notifyState() {
	d.stateMu.Lock()
	close(d.stateCh)
	d.stateCh = make(chan struct{})
	d.stateMu.Unlock()
}

// stateChanged returns a channel closed on the next session change.
func (d *Daemon) stateChanged() <-chan struct{} {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	return d.stateCh
}

// adapterFor resolves the session's agent adapter.
func adapterFor(info core.Session) (agent.Adapter, error) {
	a, ok := agent.Get(info.Agent)
	if !ok {
		return nil, badRequest("unknown agent %q (available: %v)", info.Agent, agent.Names())
	}
	return a, nil
}

// workspace returns the workspace with id, from the cache or the store.
func (d *Daemon) workspace(ctx context.Context, id core.WorkspaceID) (core.Workspace, error) {
	d.mu.RLock()
	w, ok := d.workspaces[id]
	d.mu.RUnlock()
	if ok {
		return w, nil
	}
	w, err := d.store.GetWorkspace(ctx, id)
	if err != nil {
		return core.Workspace{}, notFound("no such workspace %q", id)
	}
	d.mu.Lock()
	d.workspaces[id] = w
	d.mu.Unlock()
	return w, nil
}

// sessionsOf lists the live sessions of a workspace ("" means all), newest last.
func (d *Daemon) sessionsOf(ws core.WorkspaceID) []*session {
	d.mu.RLock()
	out := make([]*session, 0, len(d.sessions))
	for _, s := range d.sessions {
		if ws == "" || s.snap().WorkspaceID == ws {
			out = append(out, s)
		}
	}
	d.mu.RUnlock()
	sortSessions(out)
	return out
}
