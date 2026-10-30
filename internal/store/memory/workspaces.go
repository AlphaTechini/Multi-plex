package memory

import (
	"cmp"
	"context"
	"slices"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

// PutWorkspace inserts or replaces a workspace by ID.
func (s *Store) PutWorkspace(_ context.Context, w core.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspaces[w.ID] = cloneWorkspace(w)
	return nil
}

// GetWorkspace returns the workspace with the given ID or store.ErrNotFound.
func (s *Store) GetWorkspace(_ context.Context, id core.WorkspaceID) (core.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.workspaces[id]
	if !ok {
		return core.Workspace{}, store.ErrNotFound
	}
	return w, nil
}

// WorkspaceByRoot returns the oldest workspace registered for root, or
// store.ErrNotFound.
func (s *Store) WorkspaceByRoot(ctx context.Context, root string) (core.Workspace, error) {
	all, _ := s.ListWorkspaces(ctx)
	for _, w := range all {
		if w.Root == root {
			return w, nil
		}
	}
	return core.Workspace{}, store.ErrNotFound
}

// ListWorkspaces returns all workspaces ordered by CreatedAt, then ID.
func (s *Store) ListWorkspaces(_ context.Context) ([]core.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]core.Workspace, 0, len(s.workspaces))
	for _, w := range s.workspaces {
		out = append(out, w)
	}
	slices.SortFunc(out, func(a, b core.Workspace) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out, nil
}
