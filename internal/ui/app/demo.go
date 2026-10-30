package app

import (
	"context"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/backend/fake"
)

// RunDemo runs the full product against the in-memory fake backend: three
// sessions, an approval waiting, believable agent terminals and a scripted
// timeline. It needs no daemon and no agent CLI (`mpx --demo`). Options.Backend
// is ignored.
func RunDemo(ctx context.Context, o Options) error {
	be := fake.New(fake.WithTimeline(1500*time.Millisecond), fake.WithApprovalDelay(900*time.Millisecond))
	defer be.Close()
	o.Backend = be
	return run(ctx, o, demoProbe)
}

// demoProbe supplies the sidebar's steering files and counts without
// touching the disk, so the demo looks the same everywhere.
func demoProbe(core.Workspace) workspaceInfo {
	return workspaceInfo{
		Steering: []steeringItem{
			{Label: "AGENTS.md", Path: "AGENTS.md"},
			{Label: "CLAUDE.md", Path: "CLAUDE.md"},
			{Label: "~/.claude/CLAUDE.md", Path: "/home/user/.claude/CLAUDE.md"},
			{Label: ".multiplex/", Path: ".multiplex", Dir: true},
		},
		Roles: 4, Skills: 6, MCP: 2,
	}
}
