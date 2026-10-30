package fake

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// Sessions lists the sessions (all workspaces when ws is "").
func (b *Backend) Sessions(ctx context.Context, ws core.WorkspaceID) ([]core.Session, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Sessions")
	out := make([]core.Session, 0, len(b.sessions))
	for _, s := range b.sessions {
		if ws == "" || s.WorkspaceID == ws {
			out = append(out, s)
		}
	}
	return out, ctxErr(ctx)
}

// Session returns one session with its summary, cards, conflicts and changes.
func (b *Backend) Session(ctx context.Context, id core.SessionID) (protocol.SessionDetail, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Session")
	s := b.session(id)
	if s == nil {
		return protocol.SessionDetail{}, notFound("session")
	}
	d := protocol.SessionDetail{
		Session:     *s,
		Notes:       append([]core.Note(nil), b.notes[id]...),
		Conflicts:   append([]core.Conflict(nil), b.conflicts[id]...),
		Changes:     changesOf(s.Slug),
		Checkpoints: 2,
	}
	if sum := b.summaries[id]; sum != nil {
		c := *sum
		d.Summary = &c
	} else {
		d.LastMessage = lastSay(b.panes[s.AgentPane])
	}
	for _, a := range b.approvals {
		if a.SessionID == id {
			d.Approvals++
		}
	}
	return d, ctxErr(ctx)
}

func lastSay(p *fakePane) string {
	if p == nil {
		return ""
	}
	for i := len(p.entries) - 1; i >= 0; i-- {
		if p.entries[i].kind == entSay {
			return p.entries[i].text
		}
	}
	return ""
}

// changesOf lists a fake session's changed files from its fake diff.
func changesOf(slug string) []vcs.FileChange {
	files, err := diff.Parse(rawDiff(slug))
	if err != nil {
		return nil
	}
	out := make([]vcs.FileChange, 0, len(files))
	for _, f := range files {
		out = append(out, vcs.FileChange{Path: f.Path(), OldPath: f.OldPath, Status: f.Status, Added: f.Added, Deleted: f.Deleted})
		if f.Status != vcs.Renamed {
			out[len(out)-1].OldPath = ""
		}
	}
	return out
}

func rawDiff(slug string) string {
	var sb strings.Builder
	for _, f := range sessionFiles(slug) {
		sb.WriteString(f.raw())
	}
	return sb.String()
}

// CreateSession adds a session in the starting state; it starts working
// shortly after, with a scripted opening in its agent terminal.
func (b *Backend) CreateSession(ctx context.Context, p protocol.SessionCreateParams) (core.Session, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("CreateSession")
	if err := ctxErr(ctx); err != nil {
		return core.Session{}, err
	}
	now := b.now()
	slug := b.uniqueSlug(p.Name, p.Task)
	role := p.Role
	if role == "" {
		role = "implementer"
	}
	base := p.BaseBranch
	if base == "" {
		base = "main"
	}
	id := core.SessionID(b.nextID("ses"))
	paneID := core.PaneID(b.nextID("pane"))
	s := core.Session{
		ID: id, WorkspaceID: b.ws.ID, Slug: slug, Task: p.Task, Role: role, Agent: orDefault(p.Agent, "claude-code"),
		BaseBranch: base, Branch: "mpx/" + slug, WorktreePath: b.ws.Root + "/.multiplex/worktrees/" + slug,
		State: core.StateStarting, StateDetail: "creating worktree", AgentPane: paneID, CreatedAt: now, UpdatedAt: now,
	}
	b.sessions = append(b.sessions, s)
	pane := newAgentPane(s, b.now)
	pane.entries = []entry{{kind: entUser, text: p.Task}}
	if p.Cols > 0 && p.Rows > 0 {
		pane.cols, pane.rows = p.Cols, p.Rows
	}
	b.panes[paneID] = pane
	b.emit(protocol.TopicSession, s)
	time.AfterFunc(600*time.Millisecond, func() { b.startWorking(id) })
	return s, nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// startWorking moves a starting session to working and opens its transcript.
func (b *Backend) startWorking(id core.SessionID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.session(id)
	if s == nil || s.State != core.StateStarting {
		return
	}
	s.State, s.StateDetail, s.UpdatedAt = core.StateWorking, "", b.now()
	p := b.panes[s.AgentPane]
	p.working, p.startedT = true, b.now()
	p.entries = append(p.entries, entry{kind: entTool, name: "Read", arg: "AGENTS.md", result: "Read 31 lines"})
	b.emit(protocol.TopicSession, *s)
	b.pushFrameLocked(p, false)
}

// uniqueSlug derives a slug from the name or task and avoids duplicates.
func (b *Backend) uniqueSlug(name, task string) string {
	src := name
	if src == "" {
		src = task
	}
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(src), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		words = append(words, w)
		if len(words) == 3 {
			break
		}
	}
	slug := strings.Join(words, "-")
	if slug == "" {
		slug = "session"
	}
	base, n := slug, 2
	for b.slugTaken(slug) {
		slug = base + "-" + itoa(n)
		n++
	}
	return slug
}

func (b *Backend) slugTaken(slug string) bool {
	for _, s := range b.sessions {
		if s.Slug == slug {
			return true
		}
	}
	return false
}

// StopSession ends the agent and leaves the session idle.
func (b *Backend) StopSession(ctx context.Context, id core.SessionID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("StopSession")
	s := b.session(id)
	if s == nil {
		return notFound("session")
	}
	s.State, s.StateDetail, s.UpdatedAt = core.StateIdle, "stopped", b.now()
	if p := b.panes[s.AgentPane]; p != nil {
		p.working, p.exited, p.code = false, true, 0
		b.emit(protocol.TopicPaneExited, p.info())
		b.pushFrameLocked(p, false)
	}
	b.emit(protocol.TopicSession, *s)
	return ctxErr(ctx)
}

// RestartSession starts a stopped session's agent again.
func (b *Backend) RestartSession(ctx context.Context, id core.SessionID) (core.Session, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("RestartSession")
	s := b.session(id)
	if s == nil {
		return core.Session{}, notFound("session")
	}
	s.State, s.StateDetail, s.UpdatedAt = core.StateWorking, "", b.now()
	if p := b.panes[s.AgentPane]; p != nil {
		p.exited, p.working, p.startedT = false, true, b.now()
		p.entries = append(p.entries, entry{kind: entSay, text: "Restarted. Picking up where I left off."})
		b.pushFrameLocked(p, true)
	}
	b.emit(protocol.TopicSession, *s)
	return *s, ctxErr(ctx)
}

// RemoveSession deletes a session.
func (b *Backend) RemoveSession(ctx context.Context, id core.SessionID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("RemoveSession")
	for i := range b.sessions {
		if b.sessions[i].ID == id {
			delete(b.panes, b.sessions[i].AgentPane)
			b.sessions = append(b.sessions[:i], b.sessions[i+1:]...)
			b.emit(protocol.TopicSessionRemoved, protocol.SessionRef{SessionID: id})
			return ctxErr(ctx)
		}
	}
	return notFound("session")
}

// MarkSeen clears the unseen flag.
func (b *Backend) MarkSeen(ctx context.Context, id core.SessionID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("MarkSeen")
	s := b.session(id)
	if s == nil {
		return notFound("session")
	}
	if s.Unseen {
		s.Unseen = false
		b.emit(protocol.TopicSession, *s)
	}
	return ctxErr(ctx)
}

// SetFollow toggles follow mode.
func (b *Backend) SetFollow(ctx context.Context, id core.SessionID, on bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("SetFollow")
	s := b.session(id)
	if s == nil {
		return notFound("session")
	}
	s.Follow = on
	b.emit(protocol.TopicSession, *s)
	return ctxErr(ctx)
}

// Message types a steering message into the agent's terminal.
func (b *Backend) Message(ctx context.Context, id core.SessionID, text string, interrupt bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Message")
	s := b.session(id)
	if s == nil {
		return notFound("session")
	}
	b.userMessageLocked(s, text)
	return ctxErr(ctx)
}

// userMessageLocked appends a user message and makes the agent work on it.
func (b *Backend) userMessageLocked(s *core.Session, text string) {
	p := b.panes[s.AgentPane]
	if p == nil {
		return
	}
	p.entries = append(p.entries, entry{kind: entUser, text: text})
	p.working, p.startedT = true, b.now()
	if s.State != core.StateWorking {
		s.State, s.StateDetail, s.UpdatedAt = core.StateWorking, "", b.now()
		b.emit(protocol.TopicSession, *s)
	}
	b.pushFrameLocked(p, false)
	id := s.ID
	time.AfterFunc(2500*time.Millisecond, func() { b.agentReply(id) })
}

// agentReply is the scripted answer to a user message.
func (b *Backend) agentReply(id core.SessionID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.session(id)
	if s == nil {
		return
	}
	p := b.panes[s.AgentPane]
	if p == nil || p.exited || !p.working {
		return
	}
	p.entries = append(p.entries, entry{kind: entSay, text: "Understood. I will take that into account in the next step."})
	b.pushFrameLocked(p, false)
}

// Interrupt stops the agent's current turn.
func (b *Backend) Interrupt(ctx context.Context, id core.SessionID) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Interrupt")
	s := b.session(id)
	if s == nil {
		return notFound("session")
	}
	if p := b.panes[s.AgentPane]; p != nil && p.working {
		p.working = false
		p.entries = append(p.entries, entry{kind: entSay, text: "Interrupted. Tell me what to do differently."})
		b.pushFrameLocked(p, false)
	}
	return ctxErr(ctx)
}
