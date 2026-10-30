package kit

import (
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// ButtonRow renders buttons left to right with two cells between them,
// dropping buttons from the end that do not fit in width (width <= 0 means
// no limit). The result has its natural width.
func ButtonRow(t *theme.Theme, buttons []ButtonOptions, width int) string {
	out := ""
	for _, b := range buttons {
		next := Button(t, b)
		if out != "" {
			next = "  " + next
		}
		if width > 0 && stringWidth(out)+stringWidth(next) > width {
			break
		}
		out += next
	}
	return out
}
