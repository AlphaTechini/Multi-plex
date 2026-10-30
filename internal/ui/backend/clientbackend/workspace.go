package clientbackend

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// OpenWorkspace opens (or finds) the workspace containing dir, and makes sure
// the event subscription is active.
func (b *Backend) OpenWorkspace(ctx context.Context, dir string) (core.Workspace, error) {
	var ws core.Workspace
	if err := b.call(ctx, protocol.MethodWorkspaceOpen, protocol.WorkspaceOpenParams{Dir: dir}, &ws); err != nil {
		return ws, err
	}
	return ws, b.Subscribe(ctx)
}

// Specs lists the specs of a workspace.
func (b *Backend) Specs(ctx context.Context, ws core.WorkspaceID) ([]protocol.SpecInfo, error) {
	var out []protocol.SpecInfo
	err := b.call(ctx, protocol.MethodSpecList, protocol.WorkspaceRef{WorkspaceID: ws}, &out)
	return out, err
}

// Spec reads one spec's three files.
func (b *Backend) Spec(ctx context.Context, ws core.WorkspaceID, slug string) (protocol.SpecDoc, error) {
	var out protocol.SpecDoc
	err := b.call(ctx, protocol.MethodSpecGet, protocol.SpecGetParams{WorkspaceID: ws, Slug: slug}, &out)
	return out, err
}
