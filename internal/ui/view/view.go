// Package view defines the contract between the TUI shell (internal/ui/app)
// and the main-view modes (Overview, Diff, Code, Specs, Term). Each mode is
// a View in its own package under internal/ui/views/.
package view

import (
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// Env is what the shell gives every view. It is shared and read-only.
type Env struct {
	Theme     *theme.Theme
	Backend   backend.Backend
	Workspace core.Workspace
}

// EventMsg wraps a daemon event. The shell forwards every event to every
// view; views ignore topics they do not use.
type EventMsg backend.Event

// SessionMsg tells a view which session the main view now belongs to.
// Session is nil when no session is selected.
type SessionMsg struct{ Session *core.Session }

// FocusMsg tells a view whether it has keyboard focus.
type FocusMsg struct{ Focused bool }

// TickMsg is the shell's animation tick (spinners, fades). Frame counts up.
// It is only sent while some view reports Animating() == true.
type TickMsg struct{ Frame int }

// ToastMsg asks the shell to show a toast.
type ToastMsg struct {
	Tone  kit.Tone
	Title string
	Body  string
	Hints []kit.Hint // optional actions shown under the body ("u undo")
}

// SwitchModeMsg asks the shell to switch the main view to a mode, e.g. the
// Overview's file links open the Diff view.
type SwitchModeMsg struct {
	Mode Mode
	Path string // optional file to open
	Line int    // optional line
}

// Mode is a main-view mode.
type Mode int

// Main-view modes, in tab order.
const (
	ModeOverview Mode = iota
	ModeDiff
	ModeCode
	ModeSpecs
	ModeTerm
)

// View is one main-view mode.
type View interface {
	// Update handles messages: tea messages (keys only when focused, mouse
	// with coordinates relative to the view's top-left), and the messages
	// in this package. It never does I/O directly; it returns commands.
	Update(msg tea.Msg) (View, tea.Cmd)
	// Render draws the view at exactly width×height cells.
	Render(width, height int) string
	// Tab returns the tab label and badge (0 = none) for the tab bar.
	Tab() (label string, badge int)
	// Hints are the key hints shown in the status bar while focused.
	Hints() []kit.Hint
	// Animating reports whether the view needs TickMsg right now.
	Animating() bool
	// Capturing reports whether the view wants raw keys (e.g. a text box
	// or terminal is focused), so the shell only intercepts the prefix key.
	Capturing() bool
}
