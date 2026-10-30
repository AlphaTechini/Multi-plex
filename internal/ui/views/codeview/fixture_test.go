package codeview

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
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/syntax"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// fakeBackend implements only what the Code view calls; anything else (the
// view must never type into the agent) panics through the nil embedded
// interface.
type fakeBackend struct {
	backend.Backend
	mu      sync.Mutex
	files   map[string]string
	binary  map[string]bool
	changes []vcs.FileChange
	reads   []string
	follows []bool
	readErr error
	setErr  error
}

func (f *fakeBackend) ReadFile(_ context.Context, _ core.SessionID, path, rev string) (protocol.FileReadResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, path)
	if f.readErr != nil {
		return protocol.FileReadResult{}, f.readErr
	}
	c, ok := f.files[path]
	if !ok {
		return protocol.FileReadResult{}, errors.New("file not found: " + path)
	}
	return protocol.FileReadResult{Path: path, Content: c, Binary: f.binary[path]}, nil
}

func (f *fakeBackend) Diff(context.Context, core.SessionID) (protocol.DiffResult, error) {
	return protocol.DiffResult{Changes: f.changes}, nil
}

func (f *fakeBackend) SetFollow(_ context.Context, _ core.SessionID, on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.follows = append(f.follows, on)
	return f.setErr
}

func (f *fakeBackend) Events() <-chan backend.Event { return nil }

var epoch = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

const sessionGo = `package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// refreshWindow is how long before expiry a token is renewed.
const refreshWindow = 60 * time.Second

// Session holds the credentials of one signed-in user.
type Session struct {
	mu     sync.Mutex
	userID string
	token  string
	expiry time.Time
	client *Client
	cache  *Cache
}

// NewSession returns a session for userID that renews tokens through client.
func NewSession(userID string, client *Client, cache *Cache) *Session {
	return &Session{userID: userID, client: client, cache: cache}
}

// Refresh renews the token when it is close to expiring.
func (s *Session) Refresh(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokenValid(time.Now()) {
		return nil
	}
	tok, err := s.cache.Get(ctx, s.userID, s.client.Token)
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}
	s.metrics.Refreshes.Inc()
	s.token, s.expiry = tok.Value, tok.Expiry
	return nil
}

/* The helpers below are
   not part of the public API. */

// Close releases the client.
func (s *Session) Close() error {
	return s.client.Close()
}

// tokenValid reports whether the token outlives the refresh window.
func (s *Session) tokenValid(now time.Time) bool {
	return s.token != "" && s.expiry.Sub(now) > refreshWindow
}

func (s *Session) debug() string {
	return fmt.Sprintf("session %q: %d bytes\t[%v]", s.userID, len(s.token), 3.14)
}
`

func goFile(extra int) string {
	var b strings.Builder
	b.WriteString(sessionGo)
	for i := range extra {
		b.WriteString("func helper")
		b.WriteString(strings.Repeat("x", i%7+1))
		b.WriteString("() int { return ")
		b.WriteString(string(rune('0' + i%10)))
		b.WriteString(" }\n")
	}
	return b.String()
}

func newBackend() *fakeBackend {
	return &fakeBackend{
		files: map[string]string{
			"src/auth/session.go": sessionGo,
			"src/auth/cache.go":   "package auth\n\n// Cache holds tokens.\ntype Cache struct{}\n",
			"docs/auth.md":        "# Auth\n\nTokens are cached.\n",
			"AGENTS.md":           "# Agents\n\nBe careful.\n",
			"big.go":              goFile(700),
		},
		binary: map[string]bool{"logo.png": true},
		changes: []vcs.FileChange{
			{Path: "src/api/client.go", Status: vcs.Modified, Added: 6, Deleted: 2},
			{Path: "src/auth/cache.go", Status: vcs.Added, Added: 41},
			{Path: "src/auth/session.go", Status: vcs.Modified, Added: 24, Deleted: 9},
			{Path: "src/auth/session_test.go", Status: vcs.Modified, Added: 88, Deleted: 4},
			{Path: "src/auth/token.go", Status: vcs.Renamed, OldPath: "src/auth/tok.go", Added: 3, Deleted: 3},
			{Path: "src/worker/refresh.go", Status: vcs.Modified, Added: 5, Deleted: 5},
			{Path: "docs/auth.md", Status: vcs.Modified, Added: 12, Deleted: 1},
		},
	}
}

// newFixture returns a focused Code view on a session, with changes loaded.
func newFixture(t testing.TB, th *theme.Theme) (*model, *fakeBackend) {
	t.Helper()
	be := newBackend()
	m := New(view.Env{Theme: th, Backend: be, Workspace: core.Workspace{ID: "w1"}}).(*model)
	m.now = func() time.Time { return epoch }
	s := core.Session{ID: "s1", Slug: "auth-refactor", State: core.StateWorking}
	exec(t, m, send(m, view.SessionMsg{Session: &s}))
	send(m, view.FocusMsg{Focused: true})
	return m, be
}

func send(m *model, msg tea.Msg) tea.Cmd {
	_, cmd := m.Update(msg)
	return cmd
}

// exec runs a command and feeds every message it produces back into the
// view, returning the toasts and mode switches it asked the shell for.
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

// openFile opens a file as the shell would, and settles the highlighting.
func openFile(t testing.TB, m *model, path string, line int) {
	t.Helper()
	exec(t, m, send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: path, Line: line}))
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "ctrl+d":
		return tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	case "ctrl+u":
		return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
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

func edit(path string, start, end int) view.EventMsg {
	b, _ := json.Marshal(core.EditEvent{SessionID: "s1", Path: path, StartLine: start, EndLine: end, At: epoch})
	return view.EventMsg{Topic: protocol.TopicEdit, Data: b}
}

func plain(s string) []string { return strings.Split(ansi.Strip(s), "\n") }

var _ = syntax.NewSource
var errBoom = errors.New("boom")
