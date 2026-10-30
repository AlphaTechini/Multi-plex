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
// When the options do not fit in Width, Choice shows the run of options
// around the selected one, with ‹ and › marking options cut off on either
// side, so the selection is always visible.
func Choice(t *theme.Theme, o ChoiceOptions) string {
	first, last := 0, len(o.Options)-1
	if o.Width > 0 && choiceWidth(o.Options, first, last) > o.Width && len(o.Options) > 0 {
		first, last = choiceWindow(o.Options, o.Selected, o.Width)
	}
	var b strings.Builder
	if first > 0 {
		b.WriteString(fg(t.Text.Faint).paint(t.Glyphs.ChevronLeft + " "))
	}
	for i := first; i <= last && i < len(o.Options); i++ {
		opt := o.Options[i]
		if i > first {
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
	if last < len(o.Options)-1 {
		b.WriteString(fg(t.Text.Faint).paint(" " + t.Glyphs.ChevronRight))
	}
	if o.Width > 0 {
		return Fit(b.String(), o.Width)
	}
	return b.String()
}

// choiceWidth is the natural width of options first..last.
func choiceWidth(opts []string, first, last int) int {
	w := 0
	for i := first; i <= last; i++ {
		if i > first {
			w++
		}
		w += stringWidth(opts[i]) + 2
	}
	return w
}

// choiceWindow picks the widest run of options around sel that fits in w,
// counting two cells for each ‹ or › marker it needs.
func choiceWindow(opts []string, sel, w int) (first, last int) {
	sel = min(max(sel, 0), len(opts)-1)
	first, last = sel, sel
	fits := func(f, l int) bool {
		n := choiceWidth(opts, f, l)
		if f > 0 {
			n += 2
		}
		if l < len(opts)-1 {
			n += 2
		}
		return n <= w
	}
	for {
		grew := false
		if last+1 < len(opts) && fits(first, last+1) {
			last++
			grew = true
		}
		if first > 0 && fits(first-1, last) {
			first--
			grew = true
		}
		if !grew {
			return first, last
		}
	}
}
