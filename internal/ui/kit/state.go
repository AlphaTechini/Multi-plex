package kit

import (
	"image/color"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// State is a session state as the UI shows it (docs/05). Screens map the
// core session state onto it.
type State int

// Session states.
const (
	// StateIdle is waiting for a new task.
	StateIdle State = iota
	// StateStarting is creating the worktree or launching the agent.
	StateStarting
	// StateWorking is running a turn.
	StateWorking
	// StateBlocked is waiting for the user (approval or decision).
	StateBlocked
	// StateDone is finished and not yet viewed.
	StateDone
	// StateError is crashed or failed setup.
	StateError
)

// String returns the lower-case label shown next to the state dot.
func (s State) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateWorking:
		return "working"
	case StateBlocked:
		return "blocked"
	case StateDone:
		return "done"
	case StateError:
		return "error"
	default:
		return "idle"
	}
}

// Color returns the state's colour in t.
func (s State) Color(t *theme.Theme) color.Color {
	switch s {
	case StateStarting:
		return t.State.Starting
	case StateWorking:
		return t.State.Working
	case StateBlocked:
		return t.State.Blocked
	case StateDone:
		return t.State.Done
	case StateError:
		return t.State.Error
	default:
		return t.State.Idle
	}
}

// Loud reports whether the state needs the user's attention (blocked, error).
// Loud states colour their label too, not just the dot.
func (s State) Loud() bool { return s == StateBlocked || s == StateError }

// glyph returns the state's symbol; frame picks the spinner frame for working.
func (s State) glyph(t *theme.Theme, frame int) string {
	g := t.Glyphs
	switch s {
	case StateStarting:
		return g.Starting
	case StateWorking:
		if t.Motion.Reduce || len(g.Spinner) == 0 {
			return g.Working
		}
		if frame < 0 {
			frame = -frame
		}
		return g.Spinner[frame%len(g.Spinner)]
	case StateBlocked:
		return g.Blocked
	case StateDone:
		return g.Done
	case StateError:
		return g.Error
	default:
		return g.Idle
	}
}
