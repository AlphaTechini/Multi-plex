// Command mpx-hook is the agent hook entry point on its own: the same code
// as `mpx hook`, in a small binary that links only what the hook path needs.
// Agent CLIs run it on every tool call, so its startup time is the floor of
// the hook latency budget (docs/03-performance.md). The daemon points agents
// at it when it sits next to mpx, and falls back to `mpx hook` otherwise.
package main

import (
	"os"

	"github.com/alphatechini/multi-plex/internal/hookcmd"

	_ "github.com/alphatechini/multi-plex/internal/agent/claudecode"
	_ "github.com/alphatechini/multi-plex/internal/agent/shell"
)

func main() {
	os.Exit(hookcmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}
