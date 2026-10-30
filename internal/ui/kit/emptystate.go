package kit

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// EmptyStateOptions describes an EmptyState.
type EmptyStateOptions struct {
	// Width and Height are the exact size of the area to fill.
	Width, Height int
	// Eyebrow is a small muted line above the title (a wordmark or context).
	Eyebrow string
	// Title is the headline ("No sessions yet").
	Title string
	// Body is one or two sentences, wrapped to at most 50 cells.
	Body string
	// Hints are the two or three keys needed to start, shown as key caps.
	Hints []Hint
	// Bg is the surface; nil means t.Surface.Base.
	Bg color.Color
}

// EmptyState renders a calm, centred welcome block for an empty region: an
// eyebrow, a title, a short body and a column of key caps with labels.
func EmptyState(t *theme.Theme, o EmptyStateOptions) string {
	bg := o.Bg
	if bg == nil {
		bg = t.Surface.Base
	}
	w, h := o.Width, o.Height
	if w <= 0 || h <= 0 {
		return Backfill(FitBlock("", w, h), bg)
	}
	measure := min(50, w-4)
	var block []string
	if o.Eyebrow != "" {
		block = append(block, fg(t.Text.Faint).paint(o.Eyebrow), "")
	}
	if o.Title != "" {
		block = append(block, ink{fg: t.Text.Strong, bold: true}.paint(o.Title))
	}
	if o.Body != "" && measure > 0 {
		if o.Title != "" {
			block = append(block, "")
		}
		for _, l := range wrap(o.Body, measure) {
			block = append(block, fg(t.Text.Secondary).paint(strings.TrimSpace(l)))
		}
	}
	if len(o.Hints) > 0 {
		block = append(block, "")
		caps := make([]string, len(o.Hints))
		cw := 0
		for i, h := range o.Hints {
			caps[i] = Keycap(t, h.Keys)
			cw = max(cw, ansi.StringWidth(caps[i]))
		}
		lw := 0
		for _, h := range o.Hints {
			lw = max(lw, ansi.StringWidth(h.Label))
		}
		for i, h := range o.Hints {
			line := PadLeft(caps[i], cw) + "  " + PadRight(fg(t.Text.Secondary).paint(h.Label), lw)
			block = append(block, line, "")
		}
		block = block[:len(block)-1]
	}
	top := max((h-len(block))*2/5, 0)
	out := make([]string, h)
	for i := range h {
		line := ""
		if j := i - top; j >= 0 && j < len(block) {
			line = block[j]
		}
		out[i] = Backfill(Center(line, w), bg)
	}
	return strings.Join(out, "\n")
}
