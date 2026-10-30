package kit

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// Segment is one item in the status bar.
type Segment struct {
	// Text is the segment text (may be pre-styled, for example a StateBadge).
	Text string
	// Tone colours plain text.
	Tone Tone
	// Bold sets bold text.
	Bold bool
	// Pill draws the segment as a filled pill in its tone colour with dark
	// text. Reserve it for things that need the user (pending approvals).
	Pill bool
}

// StatusBarOptions describes a StatusBar.
type StatusBarOptions struct {
	// Width is the exact width in cells.
	Width int
	// Left segments, in priority order: when space runs out they are dropped
	// from the end, but the first is always kept.
	Left []Segment
	// Right segments, shown before the hints. They outrank every left
	// segment but the first, so attention pills survive narrow terminals.
	Right []Segment
	// Hints are key hints at the far right, dropped first.
	Hints []Hint
}

// StatusBar renders the bottom bar on the sunken surface: left segments
// separated by wide gaps, then right segments and key hints at the far
// right. When space runs out it drops, in order: hints (from the end), left
// segments after the first (from the end), right segments, and finally
// truncates what is left.
func StatusBar(t *theme.Theme, o StatusBarOptions) string {
	w := o.Width
	if w <= 0 {
		return ""
	}
	bg := t.Surface.Sunken
	left := o.Left
	right := joinSegments(t, o.Right)
	fits := func(l, r string) bool { return ansi.StringWidth(l)+ansi.StringWidth(r)+3 <= w }

	l := " " + joinSegments(t, left)
	if right != "" && len(o.Hints) > 0 {
		room := w - ansi.StringWidth(l) - ansi.StringWidth(right) - 7
		if room > 0 {
			if h := strings.TrimRight(KeyHints(t, o.Hints, room), " "); h != "" {
				return Backfill(spread(l, right+"   "+h+" ", w), bg)
			}
		}
	}
	if right == "" && len(o.Hints) > 0 {
		room := w - ansi.StringWidth(l) - 4
		if h := strings.TrimRight(KeyHints(t, o.Hints, max(room, 0)), " "); h != "" && room > 0 {
			return Backfill(spread(l, h+" ", w), bg)
		}
	}
	for right != "" && len(left) > 1 && !fits(l, right+" ") {
		left = left[:len(left)-1]
		l = " " + joinSegments(t, left)
	}
	if right != "" && fits(l, right+" ") {
		return Backfill(spread(l, right+" ", w), bg)
	}
	return Backfill(Fit(l, w), bg)
}

// joinSegments renders segments separated by three spaces.
func joinSegments(t *theme.Theme, segs []Segment) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		if s.Text == "" {
			continue
		}
		c := ToneColor(t, s.Tone)
		if s.Pill {
			parts = append(parts, ink{fg: t.Text.OnState, bg: c, bold: true}.paint(" "+s.Text+" "))
			continue
		}
		parts = append(parts, ink{fg: c, bold: s.Bold}.paint(s.Text))
	}
	return strings.Join(parts, "   ")
}
