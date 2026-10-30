package daemon

import (
	"context"
	"log/slog"
	"time"

	"github.com/alphatechini/multi-plex/internal/approval/broker"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/pty"
	"github.com/alphatechini/multi-plex/internal/store"
	"github.com/alphatechini/multi-plex/internal/vcs"
	"github.com/alphatechini/multi-plex/internal/vt"
)

// Remotes is the optional part of version control the push broker needs:
// remote lookup and pushing. It is separate from vcs.VCS because only the
// daemon's push path uses it.
type Remotes interface {
	// HasRemote reports whether the repository has a remote with this name.
	HasRemote(ctx context.Context, repo, name string) bool
	// Push pushes branch to remote from repo with the user's own
	// environment and credentials, and returns the tool's output.
	Push(ctx context.Context, repo, remote, branch string) (output string, err error)
}

// Deps is everything the daemon needs. All fields except Remotes, Logger,
// ApproverName and Now are required.
type Deps struct {
	Store    store.Store
	VCS      vcs.VCS
	Spawner  pty.Spawner
	Emulator vt.Factory
	// Broker coordinates approvals. The daemon sets its OnPending and
	// OnResolved callbacks and writes the audit log itself, so wire it with a
	// nil audit function.
	Broker *broker.Broker
	// Remotes enables the push broker; nil disables `mpx push` and the
	// push-URL protection.
	Remotes  Remotes
	Paths    config.Paths
	Settings config.Settings
	// MpxPath is the absolute path of the mpx binary, written into hook and
	// MCP commands.
	MpxPath string
	Version string
	// ApproverName is the active approval method shown in the hello reply.
	ApproverName string
	// Logger receives daemon logs. Nil opens Paths.LogDir/daemon.log.
	Logger *slog.Logger
	// Now returns the current time; nil means time.Now.
	Now func() time.Time
}
