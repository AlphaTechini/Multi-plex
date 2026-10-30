package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// Run starts the TUI and blocks until the user quits or ctx ends. The
// terminal's colour profile picks the theme variant; alt screen, mouse cell
// motion, focus reporting and the window title are declared by the View.
// Quitting detaches only: the daemon and its sessions keep running.
func Run(ctx context.Context, o Options) error {
	return run(ctx, o, nil)
}

func run(ctx context.Context, o Options, probe func(core.Workspace) workspaceInfo, extra ...tea.ProgramOption) error {
	if o.Backend == nil {
		return errors.New("app: Options.Backend is required")
	}
	th, err := buildTheme(o, colorprofile.Detect(os.Stdout, os.Environ()))
	if err != nil {
		return err
	}
	m := newModel(o, th, probe)
	opts := append([]tea.ProgramOption{tea.WithContext(ctx), tea.WithColorProfile(th.Profile)}, extra...)
	p := tea.NewProgram(m, opts...)
	_, err = p.Run()
	if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
		return nil
	}
	return err
}

// buildTheme picks the named theme for the terminal's profile and applies the
// glyph and motion options.
func buildTheme(o Options, profile colorprofile.Profile) (*theme.Theme, error) {
	name := o.Theme
	if name == "" {
		name = theme.DarkPlusName
	}
	th, ok := theme.ByName(name)
	if !ok {
		return nil, fmt.Errorf("unknown theme %q (have %v)", name, theme.Names())
	}
	th = th.For(profile)
	if o.ASCII {
		th = th.WithGlyphs(theme.ASCIIGlyphs())
	}
	return th.WithReducedMotion(o.ReduceMotion), nil
}
