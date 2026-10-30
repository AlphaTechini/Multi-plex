package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alphatechini/multi-plex/internal/bench"
	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/daemon"
	"github.com/alphatechini/multi-plex/internal/ui/app"
	"github.com/alphatechini/multi-plex/internal/ui/backend/clientbackend"
)

// runTUI starts the terminal UI: it makes sure the daemon is running,
// attaches to it, and opens the workspace containing the current directory.
// With --demo it runs against built-in sample data, with no daemon or agent.
func runTUI(args []string) error {
	fs := flag.NewFlagSet("mpx", flag.ContinueOnError)
	demo := fs.Bool("demo", false, "show the UI with sample sessions; no daemon or agent CLI needed")
	ascii := fs.Bool("ascii", false, "use ASCII glyphs instead of Unicode")
	dir := fs.String("C", "", "open the workspace containing this directory (default: current directory)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	settings, err := config.LoadSettings(filepath.Join(paths.ConfigDir, "settings.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mpx: %v (using defaults)\n", err)
		settings = config.DefaultSettings()
	}
	wd := *dir
	if wd == "" {
		if wd, err = os.Getwd(); err != nil {
			return err
		}
	}
	opts := app.Options{
		WorkspaceDir:    wd,
		Theme:           settings.Theme,
		PrefixKey:       settings.PrefixKey,
		ReduceMotion:    settings.ReduceMotion,
		ASCII:           *ascii,
		PanelWidth:      settings.AgentPanelWidth,
		PanelMinCols:    settings.AgentPanelMinCols,
		BenchFirstFrame: os.Getenv(bench.EnvFirstFrame) == "1",
	}

	ctx := context.Background()
	if *demo {
		return app.RunDemo(ctx, opts)
	}

	addr, err := daemon.EnsureDaemon(ctx)
	if err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	c, err := client.Dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("attach to daemon at %s: %w", addr, err)
	}
	be := clientbackend.New(c)
	defer be.Close()
	opts.Backend = be
	return app.Run(ctx, opts)
}
