package kit

import "github.com/alphatechini/multi-plex/internal/ui/theme"

// StateDot renders the one-cell state symbol in the state's colour. For
// StateWorking, frame selects the spinner frame (advance it every
// t.Motion.SpinnerInterval); with reduced motion a static glyph is used.
func StateDot(t *theme.Theme, s State, frame int) string {
	return fg(s.Color(t)).paint(s.glyph(t, frame))
}
