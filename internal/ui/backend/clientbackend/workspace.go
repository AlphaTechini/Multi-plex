package clientbackend

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// OpenWorkspace opens (or finds) the workspace containing dir, and makes sure
// the event subscription is active. It also says hello to the daemon once,
// which tells the backend the approval method.
func (b *Backend) OpenWorkspace(ctx context.Context, dir string) (core.Workspace, error) {
	var ws core.Workspace
	if err := b.call(ctx, protocol.MethodWorkspaceOpen, protocol.WorkspaceOpenParams{Dir: dir}, &ws); err != nil {
		return ws, err
	}
	b.helloOnce.Do(func() {
		// Best effort: without it approvals fall back to the method sent
		// with each approval.pending event.
		if h, err := b.c.Hello(ctx, "tui"); err == nil {
			b.approver.Store(&h.Approver)
		}
	})
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

// Agents lists the agent adapters the daemon knows and whether each is installed.
func (b *Backend) Agents(ctx context.Context) ([]protocol.AgentInfo, error) {
	var out []protocol.AgentInfo
	err := b.call(ctx, protocol.MethodAgentList, nil, &out)
	return out, err
}

// Roles lists the roles visible to a workspace.
func (b *Backend) Roles(ctx context.Context, ws core.WorkspaceID) ([]protocol.RoleInfo, error) {
	var out []protocol.RoleInfo
	err := b.call(ctx, protocol.MethodRoleList, protocol.WorkspaceRef{WorkspaceID: ws}, &out)
	return out, err
}

// ApprovalMethod reports the verifier the daemon uses (its hello result), or
// "" before the first OpenWorkspace.
func (b *Backend) ApprovalMethod() string {
	if m := b.approver.Load(); m != nil {
		return *m
	}
	return ""
}
