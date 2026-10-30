// Package claudecode is the agent adapter for Claude Code, Multiplex's main
// integration in v1 (docs/01-architecture.md "Integration points"). It writes
// the per-session hook and MCP configuration, builds the launch command line,
// and translates Claude Code's hook JSON to and from neutral events.
//
// Every JSON field name and flag spelling here comes from
// /tmp/claude-0/research/claude-code.md, checked against Claude Code 2.1.314.
package claudecode

import (
	"runtime"

	"github.com/alphatechini/multi-plex/internal/agent"
)

// Name is the registry name of this adapter.
const Name = "claude-code"

// PermissionPrefix starts the Message of the HookNotification event produced
// by Claude Code's PermissionRequest hook (and by a Notification of type
// permission_prompt). It is the immediate "blocked" signal: the daemon sets
// the session to blocked when it sees a notification whose Message starts
// with it.
const PermissionPrefix = "permission:"

func init() { agent.Register(Adapter{}) }

// Adapter runs and talks to Claude Code.
type Adapter struct{}

var _ agent.Adapter = Adapter{}

// Name returns "claude-code".
func (Adapter) Name() string { return Name }

// DisplayName returns the name shown in the UI.
func (Adapter) DisplayName() string { return "Claude Code" }

// Capabilities reports what Claude Code supports here. Its own sandbox exists
// on macOS, Linux and WSL2 but not on native Windows (docs/04-safety.md).
func (Adapter) Capabilities() agent.Capabilities {
	return agent.Capabilities{
		Hooks:         true,
		MCP:           true,
		Sandbox:       runtime.GOOS != "windows",
		SideQuestions: true,
		Roles:         true,
	}
}
