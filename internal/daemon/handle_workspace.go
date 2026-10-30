package daemon

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/store"
)

// hWorkspaceOpen finds or creates the workspace for the repository
// containing Dir.
func (d *Daemon) hWorkspaceOpen(ctx context.Context, _ *conn, p protocol.WorkspaceOpenParams) (any, error) {
	if p.Dir == "" {
		return nil, badRequest("dir is required")
	}
	abs, err := filepath.Abs(p.Dir)
	if err != nil {
		return nil, badRequest("bad dir: %v", err)
	}
	root, err := d.vcs.RepoRoot(ctx, abs)
	if err != nil {
		return nil, badRequest("%s is not inside a git repository: %v", abs, err)
	}
	root = filepath.Clean(root)

	d.createMu.Lock()
	defer d.createMu.Unlock()
	ws, err := d.store.WorkspaceByRoot(ctx, root)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		ws = core.Workspace{
			ID:        core.WorkspaceID(core.NewID("ws")),
			Root:      root,
			Name:      filepath.Base(root),
			CreatedAt: d.now(),
		}
		if err := d.store.PutWorkspace(ctx, ws); err != nil {
			return nil, err
		}
	}
	d.mu.Lock()
	d.workspaces[ws.ID] = ws
	d.mu.Unlock()
	return ws, nil
}

// hWorkspaceList lists the known workspaces.
func (d *Daemon) hWorkspaceList(ctx context.Context, _ *conn, _ protocol.Empty) (any, error) {
	list, err := d.store.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []core.Workspace{}
	}
	return list, nil
}
