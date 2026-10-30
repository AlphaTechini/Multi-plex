package client

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// OpenWorkspace opens (or finds) the workspace of the repository containing dir.
func (c *Client) OpenWorkspace(ctx context.Context, dir string) (core.Workspace, error) {
	var ws core.Workspace
	err := c.Call(ctx, protocol.MethodWorkspaceOpen, protocol.WorkspaceOpenParams{Dir: dir}, &ws)
	return ws, err
}
