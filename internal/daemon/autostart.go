package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/transport"
)

// Autostart timings.
const (
	// probeTimeout bounds the check whether a daemon already answers.
	probeTimeout = 300 * time.Millisecond
	// startWait is how long EnsureDaemon waits for a new daemon to listen.
	startWait = 2 * time.Second
	// startPoll is the interval between dial attempts while waiting.
	startPoll = 10 * time.Millisecond
)

// EnsureDaemon returns the daemon's address, starting `mpx daemon` detached if
// nothing answers there. The new daemon's output goes to the log directory.
// It waits up to two seconds for the daemon to listen. The TUI calls this
// before attaching (docs/01-architecture.md "Binary roles").
func EnsureDaemon(ctx context.Context) (addr string, err error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return "", err
	}
	addr = config.SocketAddress(paths)
	if answers(ctx, addr) {
		return addr, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("daemon: cannot find the mpx executable: %w", err)
	}
	if err := paths.Ensure(); err != nil {
		return "", err
	}
	out, err := os.OpenFile(filepath.Join(paths.LogDir, "daemon.out"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", fmt.Errorf("daemon: open log: %w", err)
	}
	defer out.Close()

	cmd := exec.Command(exe, "daemon")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, out, out
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("daemon: start %s: %w", exe, err)
	}
	_ = cmd.Process.Release() // the daemon outlives this process

	deadline := time.Now().Add(startWait)
	for {
		if answers(ctx, addr) {
			return addr, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("daemon: started %s but nothing answered at %s within %s (see %s)",
				exe, addr, startWait, filepath.Join(paths.LogDir, "daemon.log"))
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(startPoll):
		}
	}
}

// answers reports whether something accepts connections at addr.
func answers(ctx context.Context, addr string) bool {
	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	c, err := transport.Dial(pctx, addr)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
