package sqlite

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
)

const sessionCols = `id, workspace_id, slug, task, role, agent, base_branch, branch, worktree_path,
state, state_detail, agent_pane, unseen, follow, created_at, updated_at`

func scanSession(r rowScanner) (core.Session, error) {
	var s core.Session
	var unseen, follow int
	var created, updated int64
	err := r.Scan(&s.ID, &s.WorkspaceID, &s.Slug, &s.Task, &s.Role, &s.Agent, &s.BaseBranch, &s.Branch,
		&s.WorktreePath, &s.State, &s.StateDetail, &s.AgentPane, &unseen, &follow, &created, &updated)
	if err != nil {
		return core.Session{}, err
	}
	s.Unseen, s.Follow = unseen != 0, follow != 0
	s.CreatedAt, s.UpdatedAt = fromNanos(created), fromNanos(updated)
	return s, nil
}

// PutSession inserts or replaces a session by ID.
func (s *Store) PutSession(ctx context.Context, x core.Session) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions (`+sessionCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    workspace_id = excluded.workspace_id, slug = excluded.slug, task = excluded.task, role = excluded.role,
    agent = excluded.agent, base_branch = excluded.base_branch, branch = excluded.branch,
    worktree_path = excluded.worktree_path, state = excluded.state, state_detail = excluded.state_detail,
    agent_pane = excluded.agent_pane, unseen = excluded.unseen, follow = excluded.follow,
    created_at = excluded.created_at, updated_at = excluded.updated_at`,
		x.ID, x.WorkspaceID, x.Slug, x.Task, x.Role, x.Agent, x.BaseBranch, x.Branch, x.WorktreePath,
		x.State, x.StateDetail, x.AgentPane, b2i(x.Unseen), b2i(x.Follow), nanos(x.CreatedAt), nanos(x.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: put session: %w", err)
	}
	return nil
}

// GetSession returns the session with the given ID or store.ErrNotFound.
func (s *Store) GetSession(ctx context.Context, id core.SessionID) (core.Session, error) {
	x, err := scanSession(s.db.QueryRowContext(ctx, `SELECT `+sessionCols+` FROM sessions WHERE id = ?`, id))
	return x, wrapGet("session", err)
}

// ListSessions returns the sessions of a workspace (all when ws is empty),
// ordered by CreatedAt ascending, then ID.
func (s *Store) ListSessions(ctx context.Context, ws core.WorkspaceID) ([]core.Session, error) {
	query, args := `SELECT `+sessionCols+` FROM sessions ORDER BY created_at, id`, []any(nil)
	if ws != "" {
		query = `SELECT ` + sessionCols + ` FROM sessions WHERE workspace_id = ? ORDER BY created_at, id`
		args = []any{ws}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list sessions: %w", err)
	}
	return collect(rows, scanSession, "sessions")
}
