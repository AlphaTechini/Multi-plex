package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

const workspaceCols = `id, root, name, created_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanWorkspace(r rowScanner) (core.Workspace, error) {
	var w core.Workspace
	var created int64
	if err := r.Scan(&w.ID, &w.Root, &w.Name, &created); err != nil {
		return core.Workspace{}, err
	}
	w.CreatedAt = fromNanos(created)
	return w, nil
}

// PutWorkspace inserts or replaces a workspace by ID.
func (s *Store) PutWorkspace(ctx context.Context, w core.Workspace) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO workspaces (`+workspaceCols+`) VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET root = excluded.root, name = excluded.name, created_at = excluded.created_at`,
		w.ID, w.Root, w.Name, nanos(w.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: put workspace: %w", err)
	}
	return nil
}

// GetWorkspace returns the workspace with the given ID or store.ErrNotFound.
func (s *Store) GetWorkspace(ctx context.Context, id core.WorkspaceID) (core.Workspace, error) {
	w, err := scanWorkspace(s.db.QueryRowContext(ctx, `SELECT `+workspaceCols+` FROM workspaces WHERE id = ?`, id))
	return w, wrapGet("workspace", err)
}

// WorkspaceByRoot returns the oldest workspace registered for root, or
// store.ErrNotFound.
func (s *Store) WorkspaceByRoot(ctx context.Context, root string) (core.Workspace, error) {
	w, err := scanWorkspace(s.db.QueryRowContext(ctx,
		`SELECT `+workspaceCols+` FROM workspaces WHERE root = ? ORDER BY created_at, id LIMIT 1`, root))
	return w, wrapGet("workspace", err)
}

// ListWorkspaces returns all workspaces ordered by CreatedAt, then ID.
func (s *Store) ListWorkspaces(ctx context.Context) ([]core.Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+workspaceCols+` FROM workspaces ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list workspaces: %w", err)
	}
	return collect(rows, scanWorkspace, "workspaces")
}

// wrapGet maps sql.ErrNoRows to store.ErrNotFound and wraps other errors.
func wrapGet(what string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sql.ErrNoRows):
		return store.ErrNotFound
	default:
		return fmt.Errorf("sqlite: get %s: %w", what, err)
	}
}

// collect drains rows with scan, always closing them.
func collect[T any](rows *sql.Rows, scan func(rowScanner) (T, error), what string) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan %s: %w", what, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: list %s: %w", what, err)
	}
	return out, nil
}
