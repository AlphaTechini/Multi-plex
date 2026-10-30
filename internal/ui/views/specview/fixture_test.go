package specview

import (
	"context"
	"errors"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

const requirementsMD = `# Token refresh

## Requirements

### Requirement 1: Token refresh

**User story:** As an API client, I want expired tokens refreshed automatically, so that requests do not fail mid-session.

**Acceptance criteria**
1. WHEN a request is made with a token that expires within 60 seconds, THE SYSTEM SHALL refresh the token before sending the request.
2. IF the refresh fails, THEN THE SYSTEM SHALL return an authentication error and SHALL NOT retry more than once.

### Requirement 2: Logging

**User story:** As an operator, I want refresh events logged, so that I can audit them.

**Acceptance criteria**
1. WHEN a refresh succeeds, THE SYSTEM SHALL log the token family and expiry time
   without logging the token itself.
`

const designMD = `# Design

## Architecture

The cache lives in ` + "`internal/auth/cache.go`" + ` and is **per-user**. See [the ADR](docs/adr-12.md).

- A token store interface
  - in-memory implementation
  - sqlite implementation
- Refresh before expiry, **never** after

> Keep the lookup lock-free: it runs on every request.

| Field | Type | Notes |
|---|---|---|
| user_id | string | primary key |
| expiry | time.Time | UTC |

` + "```go" + `
func (c *Cache) Get(ctx context.Context, user string) (Token, error)
` + "```" + `

---

Done.
`

const tasksMD = `# Implementation plan

- [x] 1. Token store
  - [x] 1.1 Add store interface
    - _Requirements: R1.1_
  - [ ] 1.2 Persist expiry times so that a restart does not drop the cached tokens for users that are active
    - _Requirements: R1.1, R1.2_
- [-] 2. Refresh logic
  - [X] 2.1 Refresh before expiry
    - _Requirements: R1.1_
  - [ ] 2.2 Retry once
    - _Requirements: R1.2_
    - Details: not a requirement
* [ ] 3. Docs
  - _Requirements: R2.1_
`

type fakeBackend struct {
	backend.Backend
	mu       sync.Mutex
	specs    []protocol.SpecInfo
	docs     map[string]protocol.SpecDoc
	listErr  error
	specErr  error
	listN    int
	docReads []string
}

func (f *fakeBackend) Specs(context.Context, core.WorkspaceID) ([]protocol.SpecInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listN++
	return f.specs, f.listErr
}

func (f *fakeBackend) Spec(_ context.Context, _ core.WorkspaceID, slug string) (protocol.SpecDoc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.docReads = append(f.docReads, slug)
	if f.specErr != nil {
		return protocol.SpecDoc{}, f.specErr
	}
	d, ok := f.docs[slug]
	if !ok {
		return protocol.SpecDoc{}, errors.New("no such spec " + slug)
	}
	return d, nil
}

func newBackend() *fakeBackend {
	return &fakeBackend{
		specs: []protocol.SpecInfo{
			{Slug: "token-refresh", Title: "Token refresh", TasksDone: 3, TasksTotal: 7, CriteriaTotal: 9},
			{Slug: "api-pagination", Title: "API pagination", TasksDone: 0, TasksTotal: 5, CriteriaTotal: 4},
			{Slug: "docs-sweep", Title: "Docs sweep", TasksDone: 6, TasksTotal: 6, CriteriaTotal: 2},
		},
		docs: map[string]protocol.SpecDoc{
			"token-refresh":  {Slug: "token-refresh", Requirements: requirementsMD, Design: designMD, Tasks: tasksMD},
			"api-pagination": {Slug: "api-pagination", Requirements: "# Pagination\n"},
			"docs-sweep":     {Slug: "docs-sweep"},
		},
	}
}

func newFixture(t testing.TB, th *theme.Theme) (*model, *fakeBackend) {
	t.Helper()
	be := newBackend()
	m := New(view.Env{Theme: th, Backend: be, Workspace: core.Workspace{ID: "w1"}}).(*model)
	exec(t, m, send(m, view.FocusMsg{Focused: true}))
	return m, be
}

func send(m *model, msg tea.Msg) tea.Cmd {
	_, cmd := m.Update(msg)
	return cmd
}

func exec(t testing.TB, m *model, cmd tea.Cmd) {
	t.Helper()
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
		default:
			run(send(m, msg))
		}
	}
	run(cmd)
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
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	}
	r := []rune(s)
	return tea.KeyPressMsg{Code: r[0], Text: s}
}

var errBoom = errors.New("boom")
