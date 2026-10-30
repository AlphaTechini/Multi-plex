package fake

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// OpenWorkspace returns the seeded workspace.
func (b *Backend) OpenWorkspace(ctx context.Context, dir string) (core.Workspace, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("OpenWorkspace")
	return b.ws, ctxErr(ctx)
}

// Specs lists the seeded specs.
func (b *Backend) Specs(ctx context.Context, ws core.WorkspaceID) ([]protocol.SpecInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Specs")
	return append([]protocol.SpecInfo(nil), b.specs...), ctxErr(ctx)
}

// Spec returns the seeded spec documents.
func (b *Backend) Spec(ctx context.Context, ws core.WorkspaceID, slug string) (protocol.SpecDoc, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Spec")
	if slug != "token-refresh" {
		return protocol.SpecDoc{}, notFound("spec " + slug)
	}
	return specDoc, ctxErr(ctx)
}
