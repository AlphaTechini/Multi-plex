package kit

import "github.com/alphatechini/multi-plex/internal/ui/theme"

// StateBadge renders the state dot followed by its label ("⠹ working").
// Quiet states use a secondary label so only blocked and error draw the eye.
func StateBadge(t *theme.Theme, s State, frame int) string {
	label := fg(t.Text.Secondary)
	if s.Loud() {
		label = fg(s.Color(t))
	}
	return StateDot(t, s, frame) + " " + label.paint(s.String())
}
