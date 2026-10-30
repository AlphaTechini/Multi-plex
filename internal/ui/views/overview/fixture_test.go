package overview

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/store"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

var _ = store.AuditEntry{}
var _ = vcs.FileChange{}

// fakeBackend implements only what the Overview calls; any other method
// panics through the nil embedded interface, which is what a test wants.
type fakeBackend struct {
	backend.Backend
	mu       sync.Mutex
	detail   protocol.SessionDetail
	sessions []core.Session
	specs    []protocol.SpecInfo
	acts     []protocol.NoteActParams
	loads    int
	actErr   error
}

func (f *fakeBackend) Session(_ context.Context, _ core.SessionID) (protocol.SessionDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loads++
	return f.detail, nil
}

func (f *fakeBackend) Sessions(context.Context, core.WorkspaceID) ([]core.Session, error) {
	return f.sessions, nil
}

func (f *fakeBackend) Specs(context.Context, core.WorkspaceID) ([]protocol.SpecInfo, error) {
	return f.specs, nil
}

func (f *fakeBackend) ActOnNote(_ context.Context, p protocol.NoteActParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acts = append(f.acts, p)
	return f.actErr
}

func (f *fakeBackend) Events() <-chan backend.Event { return nil }

var epoch = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

// mockDetail is the data behind the docs/16 Overview mockup.
func mockDetail() protocol.SessionDetail {
	return protocol.SessionDetail{
		Session: core.Session{
			ID: "s1", Slug: "auth-refactor", Task: "Cache auth tokens per user and update the callers",
			Role: "implementer", Branch: "mpx/auth-refactor", State: core.StateWorking,
			CreatedAt: epoch.Add(-14 * time.Minute),
		},
		Summary: &core.Summary{
			SessionID:     "s1",
			Done:          []string{"Added token cache to Session.Refresh", "Moved the expiry check into tokenValid()"},
			Decisions:     []string{"Cache TTL is 5 minutes, matching the token lifetime"},
			OpenQuestions: []string{"Should the cache be per-user or global?"},
			Next:          "Update callers in api/ and worker/",
			FilesChanged: []string{"src/auth/session.go", "src/auth/cache.go", "src/auth/session_test.go",
				"src/auth/token.go", "src/api/client.go", "src/worker/refresh.go", "docs/auth.md"},
			At: epoch.Add(-time.Minute),
		},
		Notes: []core.Note{
			{ID: "n3", SessionID: "s1", Number: 3, Severity: core.SeverityInfo, At: epoch.Add(-11 * time.Minute),
				Title: "Assumed tokens expire after 5 minutes",
				Body:  "Matched the TTL to the identity provider's default token lifetime. Override with AUTH_CACHE_TTL.",
				Refs:  []string{"src/auth/cache.go:L8"}},
			{ID: "n1", SessionID: "s1", Number: 1, Severity: core.SeverityDecisionNeeded, At: epoch.Add(-2 * time.Minute),
				Title: "Cache scope: per-user or global?",
				Body:  "A per-user cache costs about 4 MB at 10k users. A global cache is smaller but would share tokens across tenants if a key is ever wrong. I am holding the caller updates until you decide.",
				Refs:  []string{"src/auth/cache.go:L12-L30"}},
			{ID: "n2", SessionID: "s1", Number: 2, Severity: core.SeverityWarning, At: epoch.Add(-6 * time.Minute),
				Title: "Changing Session.Refresh signature",
				Body:  "Refresh now takes a cache argument. Callers in api/ and worker/ need updating; api-pagination also edits src/api/client.go.",
				Refs:  []string{"src/auth/session.go:L38-L52"}},
		},
		Turn:    6,
		Reports: 6,
		Conflicts: []core.Conflict{{SessionID: "s1", OtherBranch: "mpx/api-pagination", OtherSession: "s2",
			Files: []string{"src/api/client.go"}}},
	}
}

// newFixture returns an Overview showing the mockup session, loaded.
func newFixture(t testing.TB, th *theme.Theme) (*model, *fakeBackend) {
	t.Helper()
	be := &fakeBackend{detail: mockDetail(), sessions: []core.Session{{ID: "s2", Slug: "api-pagination"}}}
	m := New(view.Env{Theme: th, Backend: be, Workspace: core.Workspace{ID: "w1"}}).(*model)
	m.now = func() time.Time { return epoch }
	s := be.detail.Session
	exec(t, m, send(m, view.SessionMsg{Session: &s}))
	m.frame = 2
	m.seen["n3"] = true
	m.checkedAt = epoch.Add(-time.Minute)
	m.spec = &protocol.SpecInfo{Slug: "token-refresh", TasksDone: 3, TasksTotal: 7, CriteriaTotal: 9, CriteriaVerified: 5}
	return m, be
}

// send delivers a message and returns the command.
func send(m *model, msg tea.Msg) tea.Cmd {
	_, cmd := m.Update(msg)
	return cmd
}

// exec runs a command and feeds every message it produces back into the
// view, recursively, returning the messages that are not for the view itself
// (toasts and mode switches).
func exec(t testing.TB, m *model, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	var out []tea.Msg
	var run func(tea.Cmd)
	run = func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch msg := c().(type) {
		case nil:
		case tea.BatchMsg:
			for _, c2 := range msg {
				run(c2)
			}
		case view.ToastMsg, view.SwitchModeMsg:
			out = append(out, msg)
		default:
			run(send(m, msg))
		}
	}
	run(cmd)
	return out
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

func typeText(t testing.TB, m *model, s string) {
	t.Helper()
	for _, r := range s {
		exec(t, m, send(m, tea.KeyPressMsg{Code: r, Text: string(r)}))
	}
}

// plain returns the rendered view without styling, as lines.
func plain(s string) []string { return strings.Split(ansi.Strip(s), "\n") }

var errBoom = errors.New("boom")

func mustJSON(v any) []byte {
	b, err := jsonMarshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

var jsonMarshal = json.Marshal
