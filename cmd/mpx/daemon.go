package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/approval/broker"
	"github.com/alphatechini/multi-plex/internal/approval/polkit"
	"github.com/alphatechini/multi-plex/internal/approval/typedcode"
	"github.com/alphatechini/multi-plex/internal/approval/windowshello"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/daemon"
	"github.com/alphatechini/multi-plex/internal/daemon/gitremote"
	"github.com/alphatechini/multi-plex/internal/pty/xptyimpl"
	"github.com/alphatechini/multi-plex/internal/store/sqlite"
	"github.com/alphatechini/multi-plex/internal/transport"
	"github.com/alphatechini/multi-plex/internal/vcs/gitcli"
	"github.com/alphatechini/multi-plex/internal/vt/charmvt"
)

// runDaemon runs the daemon in the foreground. This is the one place the real
// implementations are chosen and wired together (docs/17-code-structure.md).
func runDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	debug := fs.Bool("debug", false, "log at debug level, including hook timings ("+daemon.EnvDebug+"=1)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *debug {
		_ = os.Setenv(daemon.EnvDebug, "1")
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	if err := paths.Ensure(); err != nil {
		return err
	}
	settings, err := config.LoadSettings(filepath.Join(paths.ConfigDir, "settings.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mpx daemon: %v; using default settings\n", err)
		settings = config.DefaultSettings()
	}

	// Listen first: if a daemon already answers, leave its database alone.
	addr := config.SocketAddress(paths)
	l, err := transport.Listen(addr)
	if errors.Is(err, transport.ErrAddrInUse) {
		fmt.Printf("mpx daemon: already running at %s\n", addr)
		return nil
	}
	if err != nil {
		return err
	}

	st, err := sqlite.Open(paths.DBPath)
	if err != nil {
		_ = l.Close()
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		_ = l.Close()
		return fmt.Errorf("find own executable: %w", err)
	}

	verifiers := chooseVerifiers(settings.Approval)
	d := daemon.New(daemon.Deps{
		Store:    st,
		VCS:      gitcli.New(),
		Spawner:  xptyimpl.New(),
		Emulator: charmvt.Factory,
		// The daemon logs every decision to the audit log itself, so the
		// broker gets no audit function.
		Broker:       broker.New(verifiers, typedcode.New(), nil),
		Remotes:      gitremote.Git{},
		Paths:        paths,
		Settings:     settings,
		MpxPath:      exe,
		Version:      version,
		ApproverName: approverName(verifiers),
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Printf("mpx daemon %s listening at %s\n", version, l.Addr())
	return d.Serve(ctx, l)
}

// chooseVerifiers builds the approval methods in order of preference, honouring
// the "approval" setting: "auto" tries Windows Hello, then polkit, then falls
// back to the typed code; a method's name selects only that one.
func chooseVerifiers(setting string) []approval.Verifier {
	hello := windowshello.New(windowshello.Find())
	pk := polkit.New()
	switch setting {
	case "windows-hello":
		return []approval.Verifier{hello}
	case "polkit":
		return []approval.Verifier{pk}
	case "typed-code":
		return nil
	}
	return []approval.Verifier{hello, pk}
}

// approverName is the method the broker will use first on this machine.
func approverName(vs []approval.Verifier) string {
	for _, v := range vs {
		if v.Available(context.Background()) {
			return v.Name()
		}
	}
	return typedcode.New().Name()
}
