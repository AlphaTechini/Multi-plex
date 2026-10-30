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

// Agents lists the two adapters a real daemon ships with.
func (b *Backend) Agents(ctx context.Context) ([]protocol.AgentInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Agents")
	return []protocol.AgentInfo{
		{Name: "claude-code", DisplayName: "Claude Code", Available: true, Detail: "/usr/local/bin/claude", Default: true},
		{Name: "shell", DisplayName: "Shell", Available: true, Detail: "/bin/bash"},
	}, ctxErr(ctx)
}

// Roles lists the built-in roles.
func (b *Backend) Roles(ctx context.Context, ws core.WorkspaceID) ([]protocol.RoleInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Roles")
	return []protocol.RoleInfo{
		{Name: "implementer", Description: "Writes code against the task and runs the tests before it finishes.", Source: "builtin", Default: true},
		{Name: "orchestrator", Description: "Breaks a goal into tasks and starts sessions for them.", Source: "builtin"},
		{Name: "planner", Description: "Reads the code and writes a plan or a spec. Does not edit code.", Source: "builtin"},
		{Name: "researcher", Description: "Investigates a question and reports what it found. Read-only.", Source: "builtin"},
		{Name: "resolver", Description: "Resolves merge conflicts between sessions.", Source: "builtin"},
		{Name: "reviewer", Description: "Reviews a branch and leaves comments. Read-only.", Source: "builtin"},
	}, ctxErr(ctx)
}
