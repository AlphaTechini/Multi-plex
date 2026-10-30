package kit

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// ChoiceOptions describes a Choice.
type ChoiceOptions struct {
	// Width > 0 fits the result to exactly Width cells.
	Width int
	// Options are the labels, in order.
	Options []string
	// Selected is the chosen option's index.
	Selected int
	// Focused shows the selection in the accent tint; unfocused it uses the
	// neutral control surface.
	Focused bool
}

// Choice renders a segmented single choice ("implementer  planner
// reviewer"): the selected option is a pill, the others muted text. Every
// option has the same padding so moving the selection does not shift them.
func Choice(t *theme.Theme, o ChoiceOptions) string {
	var b strings.Builder
	for i, opt := range o.Options {
		if i > 0 {
			b.WriteString(" ")
		}
		if i == o.Selected {
			bg := t.Surface.Control
			if o.Focused {
				bg = t.Accent.Subtle
			}
			b.WriteString(ink{fg: t.Text.Strong, bg: bg, bold: true}.paint(" " + opt + " "))
			continue
		}
		b.WriteString(fg(t.Text.Muted).paint(" " + opt + " "))
	}
	if o.Width > 0 {
		return Fit(b.String(), o.Width)
	}
	return b.String()
}
