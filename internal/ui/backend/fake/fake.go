// Package fake is an in-memory backend.Backend with rich seeded data: three
// sessions (working, blocked on an approval, done), a summary, insight cards,
// a multi-file diff, specs and believable agent terminals. Every method has
// an in-memory effect, and an optional scripted timeline pushes events so a
// demo feels alive. It drives tests, previews and `mpx --demo`.
package fake

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/store"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
)

// Seeded IDs, for tests.
const (
	WorkspaceID = core.WorkspaceID("ws_multiplex")

	SessionAuth = core.SessionID("ses_auth")
	SessionAPI  = core.SessionID("ses_api")
	SessionDocs = core.SessionID("ses_docs")

	PaneAuth = core.PaneID("pane_auth")
	PaneAPI  = core.PaneID("pane_api")
	PaneDocs = core.PaneID("pane_docs")

	// ApprovalID is the seeded pending approval of the api-pagination session.
	ApprovalID = "apr_golden"
)

// Backend is the fake. It is safe for concurrent use.
type Backend struct {
	events chan backend.Event
	done   chan struct{}
	once   sync.Once

	mu        sync.Mutex
	ws        core.Workspace
	sessions  []core.Session
	summaries map[core.SessionID]*core.Summary
	notes     map[core.SessionID][]core.Note
	comments  map[core.SessionID][]core.Comment
	approvals []core.ApprovalRequest
	audit     []store.AuditEntry
	method    string // approval method reported with approval.pending
	code      string // one-time code for the typed-code method
	panes     map[core.PaneID]*fakePane
	conflicts map[core.SessionID][]core.Conflict
	specs     []protocol.SpecInfo
	seq       int
	now       func() time.Time
	delay     time.Duration

	// Calls records the name of every backend method called, for tests.
	calls []string
}

var _ backend.Backend = (*Backend)(nil)

// Option configures New.
type Option func(*config)

type config struct {
	empty    bool
	interval time.Duration
	root     string
	method   string
	now      func() time.Time
	delay    time.Duration
}

// Empty creates a backend with no sessions (the first-run state). The specs
// stay, as they live in the repository.
func Empty() Option { return func(c *config) { c.empty = true } }

// WithTimeline starts the scripted timeline: every interval one scripted step
// emits events (pane frames, a new note, a summary update, state changes).
func WithTimeline(interval time.Duration) Option { return func(c *config) { c.interval = interval } }

// WithRoot sets the workspace root OpenWorkspace reports.
func WithRoot(root string) Option { return func(c *config) { c.root = root } }

// WithApprovalMethod sets the verifier reported with approval.pending
// ("windows-hello", "touch-id", "polkit", "typed-code"). With "typed-code" the
// pending event carries a one-time code that RespondApproval requires.
func WithApprovalMethod(m string) Option { return func(c *config) { c.method = m } }

// WithApprovalDelay makes RespondApproval take d, like a human-present check
// (a Windows Hello prompt) does.
func WithApprovalDelay(d time.Duration) Option { return func(c *config) { c.delay = d } }

// WithClock sets the clock used for timestamps (tests).
func WithClock(now func() time.Time) Option { return func(c *config) { c.now = now } }

// New creates a seeded fake backend.
func New(opts ...Option) *Backend {
	c := config{root: defaultRoot, method: "windows-hello", now: time.Now}
	for _, o := range opts {
		o(&c)
	}
	b := &Backend{
		events:    make(chan backend.Event, 4096),
		done:      make(chan struct{}),
		summaries: map[core.SessionID]*core.Summary{},
		notes:     map[core.SessionID][]core.Note{},
		comments:  map[core.SessionID][]core.Comment{},
		panes:     map[core.PaneID]*fakePane{},
		conflicts: map[core.SessionID][]core.Conflict{},
		method:    c.method,
		now:       c.now,
		delay:     c.delay,
	}
	b.seed(c)
	if c.interval > 0 {
		go b.runTimeline(c.interval)
	}
	return b
}

// Events returns the event channel. It is closed by Close.
func (b *Backend) Events() <-chan backend.Event { return b.events }

// Close stops the timeline and closes the event channel.
func (b *Backend) Close() error {
	b.once.Do(func() {
		close(b.done)
		b.mu.Lock()
		close(b.events)
		b.mu.Unlock()
	})
	return nil
}

// Emit pushes an event with a JSON-encoded payload, as the daemon would.
func (b *Backend) Emit(topic string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.emitLocked(topic, data)
}

// emitLocked sends an event. Pane frames are dropped when the channel is
// full; everything else waits briefly. The caller holds b.mu, which also
// guards the channel against a concurrent Close.
func (b *Backend) emitLocked(topic string, data json.RawMessage) {
	select {
	case <-b.done:
		return
	default:
	}
	ev := backend.Event{Topic: topic, Data: data}
	select {
	case b.events <- ev:
	default:
		if topic != protocol.TopicPaneFrame {
			// Make room: the fake must not block the UI loop's writers.
			select {
			case <-b.events:
			default:
			}
			select {
			case b.events <- ev:
			default:
			}
		}
	}
}

func (b *Backend) emit(topic string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	b.emitLocked(topic, data)
}

// Calls returns the names of the backend methods called so far.
func (b *Backend) Calls() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.calls...)
}

func (b *Backend) rec(name string) { b.calls = append(b.calls, name) }

// session returns a pointer to the stored session. The caller holds b.mu.
func (b *Backend) session(id core.SessionID) *core.Session {
	for i := range b.sessions {
		if b.sessions[i].ID == id {
			return &b.sessions[i]
		}
	}
	return nil
}

func (b *Backend) nextID(prefix string) string {
	b.seq++
	return prefix + "_" + strconv.Itoa(b.seq)
}

func notFound(what string) error {
	return &protocol.Error{Code: protocol.CodeNotFound, Message: what + " not found"}
}

func ctxErr(ctx context.Context) error { return ctx.Err() }

func itoa(n int) string { return strconv.Itoa(n) }

// ApprovalMethod reports the verifier the fake daemon uses.
func (b *Backend) ApprovalMethod() string { return b.method }
