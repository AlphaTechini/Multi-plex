package fake

import (
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

const defaultRoot = "/home/user/dev/multi-plex"

// seed fills the backend with the demo scenario (docs/16's mockups).
func (b *Backend) seed(c config) {
	now := b.now()
	b.ws = core.Workspace{ID: WorkspaceID, Root: c.root, Name: "multi-plex", CreatedAt: now.Add(-72 * time.Hour)}
	b.specs = []protocol.SpecInfo{{Slug: "token-refresh", Title: "Token refresh", TasksDone: 3, TasksTotal: 7, CriteriaTotal: 9, CriteriaVerified: 5}}
	if c.empty {
		return
	}

	b.sessions = []core.Session{
		{
			ID: SessionAuth, WorkspaceID: WorkspaceID, Slug: "auth-refactor",
			Task: "Cache auth tokens per user and update the callers",
			Role: "implementer", Agent: "claude-code", BaseBranch: "main", Branch: "mpx/auth-refactor",
			WorktreePath: c.root + "/.multiplex/worktrees/auth-refactor",
			State:        core.StateWorking, AgentPane: PaneAuth, Follow: true,
			CreatedAt: now.Add(-14 * time.Minute), UpdatedAt: now.Add(-32 * time.Second),
		},
		{
			ID: SessionAPI, WorkspaceID: WorkspaceID, Slug: "api-pagination",
			Task: "Add cursor pagination to the list endpoints",
			Role: "implementer", Agent: "claude-code", BaseBranch: "main", Branch: "mpx/api-pagination",
			WorktreePath: c.root + "/.multiplex/worktrees/api-pagination",
			State:        core.StateBlocked, StateDetail: "needs approval", AgentPane: PaneAPI,
			CreatedAt: now.Add(-23 * time.Minute), UpdatedAt: now.Add(-2 * time.Minute),
		},
		{
			ID: SessionDocs, WorkspaceID: WorkspaceID, Slug: "docs-sweep",
			Task: "Update the docs for the new auth flow",
			Role: "implementer", Agent: "claude-code", BaseBranch: "main", Branch: "mpx/docs-sweep",
			WorktreePath: c.root + "/.multiplex/worktrees/docs-sweep",
			State:        core.StateDone, StateDetail: "done · 6 files", AgentPane: PaneDocs, Unseen: true,
			CreatedAt: now.Add(-41 * time.Minute), UpdatedAt: now.Add(-9 * time.Minute),
		},
	}

	b.summaries[SessionAuth] = &core.Summary{
		SessionID: SessionAuth,
		Done:      []string{"Added token cache to Session.Refresh", "Moved the expiry check into tokenValid()"},
		FilesChanged: []string{
			"src/auth/session.go", "src/auth/cache.go", "src/auth/session_test.go",
			"src/auth/token.go", "src/api/client.go", "src/worker/refresh.go", "docs/auth.md",
		},
		Decisions:     []string{"Cache TTL is 5 minutes, matching the token lifetime"},
		OpenQuestions: []string{"Should the cache be per-user or global?"},
		Next:          "Update callers in api/ and worker/",
		At:            now.Add(-32 * time.Second),
	}
	b.summaries[SessionDocs] = &core.Summary{
		SessionID:    SessionDocs,
		Done:         []string{"Rewrote the token lifetime section", "Documented the pagination parameters"},
		FilesChanged: []string{"docs/auth.md", "docs/api.md"},
		Next:         "Nothing further",
		At:           now.Add(-9 * time.Minute),
	}

	b.turns[SessionAuth], b.reports[SessionAuth] = 6, 6
	b.turns[SessionAPI] = 4
	b.turns[SessionDocs], b.reports[SessionDocs] = 5, 3

	b.notes[SessionAuth] = []core.Note{
		{
			ID: "note_1", SessionID: SessionAuth, Number: 1, Severity: core.SeverityDecisionNeeded,
			Title: "Cache scope: per-user or global?",
			Body:  "A per-user cache costs about 4 MB at 10k users. A global cache is smaller but would share tokens across tenants if a key is ever wrong. I am holding the caller updates until you decide.",
			Refs:  []string{"src/auth/cache.go:L12-L30"}, At: now.Add(-2 * time.Minute),
		},
		{
			ID: "note_2", SessionID: SessionAuth, Number: 2, Severity: core.SeverityWarning,
			Title: "Changing Session.Refresh signature",
			Body:  "Refresh now takes a cache argument. Callers in api/ and worker/ need updating; api-pagination also edits src/api/client.go.",
			Refs:  []string{"src/auth/session.go:L38-L52"}, At: now.Add(-6 * time.Minute),
		},
		{
			ID: "note_3", SessionID: SessionAuth, Number: 3, Severity: core.SeverityInfo,
			Title: "Assumed tokens expire after 5 minutes",
			Body:  "Matched the TTL to the identity provider's default token lifetime. Override with AUTH_CACHE_TTL.",
			Refs:  []string{"src/auth/cache.go:L8"}, At: now.Add(-11 * time.Minute),
		},
	}
	b.notes[SessionAPI] = []core.Note{{
		ID: "note_4", SessionID: SessionAPI, Number: 1, Severity: core.SeverityWarning,
		Title: "Needs to delete the golden test data",
		Body:  "The old golden files encode the unpaginated shape. I want to regenerate them, which deletes testdata/golden/ first.",
		Refs:  []string{"testdata/golden/session_refresh.json"}, At: now.Add(-3 * time.Minute),
	}}

	b.conflicts[SessionAuth] = []core.Conflict{{
		SessionID: SessionAuth, OtherBranch: "mpx/api-pagination", OtherSession: SessionAPI,
		Files: []string{"src/api/client.go"},
	}}

	b.approvals = []core.ApprovalRequest{{
		ID: ApprovalID, SessionID: SessionAPI, Kind: core.ApprovalCommand, Tool: "Bash",
		Command: `Remove-Item -Recurse -Force ..\..\multi-plex\testdata\golden`,
		Cwd:     `%LOCALAPPDATA%\Multiplex\worktrees\multi-plex\api-pagination`,
		Paths: []string{
			"testdata/golden/session_refresh.json", "testdata/golden/session_expiry.json",
			"testdata/golden/session_revoke.json", "testdata/golden/client_page1.json",
			"testdata/golden/client_page2.json", "testdata/golden/client_last.json",
		},
		Layer: "Layer 2 writes outside the session's worktree; Layer 4 recursive delete",
		Rule:  "rm-recursive · .multiplex/rules.toml:14",
		At:    now.Add(-2 * time.Minute),
	}}
	if b.method == "typed-code" {
		b.code = "7K3-Q9"
	}

	for _, s := range b.sessions {
		p := newAgentPane(s, b.now)
		b.panes[s.AgentPane] = p
	}
	b.panes[PaneAuth].working = true
}

// pendingEvent builds the approval.pending payload for a request.
func (b *Backend) pendingEvent(r core.ApprovalRequest) protocol.ApprovalPending {
	return protocol.ApprovalPending{Request: r, Method: b.method, Code: b.code}
}
