package kit

import (
	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// KeyHints renders hints in order, separated by three spaces, dropping whole
// hints from the end until they fit in width (a hint is never cut in the
// middle). With width > 0 the result is padded to exactly width cells; with
// width <= 0 it has its natural width.
func KeyHints(t *theme.Theme, hints []Hint, width int) string {
	const gap = "   "
	out := ""
	for _, h := range hints {
		next := KeyHint(t, h)
		if out != "" {
			next = gap + next
		}
		if width > 0 && ansi.StringWidth(out)+ansi.StringWidth(next) > width {
			break
		}
		out += next
	}
	if width > 0 {
		return PadRight(out, width)
	}
	return out
}
