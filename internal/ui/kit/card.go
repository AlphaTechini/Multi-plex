package kit

import (
	"fmt"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// CardOptions describes an insight Card (docs/08).
type CardOptions struct {
	// Width is the exact width in cells.
	Width int
	// Number is the card's number in the Overview (newest first), used by
	// keys like "1".."9".
	Number int
	// Severity sets the bar and label colour.
	Severity Severity
	// Title is the one-line headline.
	Title string
	// Body is wrapped plain text, clamped to BodyLines lines.
	Body string
	// BodyLines is the fixed number of body rows (default 2). It is fixed so
	// cards keep their height when selected or updated.
	BodyLines int
	// Refs are file references ("src/auth/session.go:L40-L52").
	Refs []string
	// Age is short relative time ("2m").
	Age string
	// Actions are the card's actions as key hints (Ask, Steer...).
	Actions []Hint
	// Selected raises the card surface and brightens its actions.
	Selected bool
	// Unread draws the title bold.
	Unread bool
}

// CardHeight returns how many lines Card renders for o: the title row, the
// body rows, a refs row when there are refs, and an actions row when there
// are actions.
func CardHeight(o CardOptions) int {
	h := 1 + bodyLines(o)
	if len(o.Refs) > 0 {
		h++
	}
	if len(o.Actions) > 0 {
		h++
	}
	return h
}

func bodyLines(o CardOptions) int {
	if o.BodyLines <= 0 {
		return 2
	}
	return o.BodyLines
}

// Card renders a numbered insight card: a severity-coloured bar down the left
// edge, the number, severity label, title and age on the first row, then the
// body, the refs and the actions. Every row is exactly Width cells.
func Card(t *theme.Theme, o CardOptions) string {
	w := o.Width
	n := CardHeight(o)
	if w <= 0 {
		return strings.Repeat("\n", n-1)
	}
	bg := t.Surface.Base
	if o.Selected {
		bg = t.Surface.Raised
	}
	sev := o.Severity.Color(t)
	bar := fg(sev).paint(t.Glyphs.Bar)
	const indent = 5 // bar, space, two-cell number, space
	cw := max(w-indent-1, 0)
	lines := make([]string, 0, n)
	emit := func(s string) {
		lines = append(lines, row(bar+" "+s, w, bg))
	}

	num := fg(t.Text.Muted).paint(fmt.Sprintf("%2d", o.Number))
	title := ink{fg: t.Text.Primary, bold: o.Unread}
	if o.Selected {
		title = ink{fg: t.Text.Strong, bold: true}
	}
	head := ink{fg: sev, bold: true}.paint(o.Severity.Label()) + "  " + title.paint(o.Title)
	emit(num + " " + spread(head, fg(t.Text.Faint).paint(o.Age), cw))

	for _, l := range clampLines(wrap(o.Body, cw), bodyLines(o), cw) {
		emit("   " + fg(t.Text.Secondary).paint(l))
	}
	if len(o.Refs) > 0 {
		emit("   " + fg(t.Accent.Base).paint(strings.Join(o.Refs, "  ")))
	}
	if len(o.Actions) > 0 {
		emit("   " + cardActions(t, o))
	}
	return strings.Join(lines, "\n")
}

// cardActions renders the action row: bright on the selected card, faint on
// the others so the list stays calm but keeps its height.
func cardActions(t *theme.Theme, o CardOptions) string {
	if o.Selected {
		return KeyHints(t, o.Actions, 0)
	}
	parts := make([]string, len(o.Actions))
	for i, a := range o.Actions {
		parts[i] = FormatKeys(t.Glyphs, a.Keys) + " " + a.Label
	}
	return fg(t.Text.Faint).paint(strings.Join(parts, "   "))
}
