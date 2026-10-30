// Package app is the Multiplex TUI shell: a Bubble Tea program that owns the
// layout (sidebar, main view, agent panel, status bar), focus, keys, overlays
// and event routing, and hosts the main-view modes through the view.View
// contract (internal/ui/view). It talks to the daemon only through a
// backend.Backend, always from tea.Cmd functions.
package app

import "github.com/alphatechini/multi-plex/internal/ui/backend"

// DefaultPrefix is the prefix key when Options.PrefixKey is empty.
const DefaultPrefix = "ctrl+\\"

// Options configures the shell.
type Options struct {
	// Backend is the daemon connection (or the fake, for the demo).
	Backend backend.Backend
	// WorkspaceDir is the directory the user started in; the daemon resolves
	// it to a workspace.
	WorkspaceDir string
	// Theme is the registry name of the colour theme ("" = Dark+).
	Theme string
	// PrefixKey is the prefix key spec ("" = ctrl+\).
	PrefixKey string
	// ReduceMotion turns animation off.
	ReduceMotion bool
	// ASCII selects the ASCII glyph set.
	ASCII bool
	// PanelWidth is the agent panel's share of the width (0 = 0.30).
	PanelWidth float64
	// PanelMinCols is the narrowest the docked agent panel may be (0 = 60).
	PanelMinCols int
	// BenchFirstFrame prints the time to the first frame on stderr and quits
	// (`mpx bench startup`, env MPX_BENCH_FIRST_FRAME=1).
	BenchFirstFrame bool
}

func (o Options) prefix() string {
	if o.PrefixKey == "" {
		return DefaultPrefix
	}
	return o.PrefixKey
}
