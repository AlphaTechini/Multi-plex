package bench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/alphatechini/multi-plex/internal/approval/broker"
	"github.com/alphatechini/multi-plex/internal/approval/typedcode"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/daemon"
	"github.com/alphatechini/multi-plex/internal/daemon/gitremote"
	"github.com/alphatechini/multi-plex/internal/pty/xptyimpl"
	"github.com/alphatechini/multi-plex/internal/store/memory"
	"github.com/alphatechini/multi-plex/internal/transport"
	"github.com/alphatechini/multi-plex/internal/vcs/gitcli"
	"github.com/alphatechini/multi-plex/internal/vt/charmvt"

	_ "github.com/alphatechini/multi-plex/internal/agent/claudecode" // adapters the daemon looks up by name
	_ "github.com/alphatechini/multi-plex/internal/agent/shell"
)

// startInProcessDaemon runs a daemon with an in-memory store on a temp
// socket and returns its address and a function that stops it.
func startInProcessDaemon() (addr string, stop func(), err error) {
	root, err := os.MkdirTemp("", "mpxbd")
	if err != nil {
		return "", nil, err
	}
	data := filepath.Join(root, "data")
	paths := config.Paths{
		DataDir: data, ConfigDir: filepath.Join(root, "config"), RuntimeDir: filepath.Join(root, "run"),
		DBPath: filepath.Join(data, "multiplex.db"), WorktreesDir: filepath.Join(data, "worktrees"), LogDir: filepath.Join(data, "logs"),
	}
	if runtime.GOOS == "windows" {
		addr = fmt.Sprintf(`\\.\pipe\mpx-bench-%d-%d`, os.Getpid(), time.Now().UnixNano())
	} else {
		addr = filepath.Join(root, "d.sock")
	}
	l, err := transport.Listen(addr)
	if err != nil {
		_ = os.RemoveAll(root)
		return "", nil, err
	}
	exe, _ := os.Executable()
	d := daemon.New(daemon.Deps{
		Store: memory.New(), VCS: gitcli.New(), Spawner: xptyimpl.New(), Emulator: charmvt.Factory,
		Broker: broker.New(nil, typedcode.New(), nil), Remotes: gitremote.Git{},
		Paths: paths, Settings: config.DefaultSettings(), MpxPath: exe, Version: "bench",
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Serve(ctx, l) }()
	stop = func() {
		cancel()
		select {
		case <-done:
		case <-time.After(20 * time.Second):
		}
		_ = os.RemoveAll(root)
	}
	return addr, stop, nil
}
