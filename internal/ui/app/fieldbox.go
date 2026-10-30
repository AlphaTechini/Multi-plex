package app

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// fieldBox draws a labelled input like kit.Field, but around lines that are
// already rendered (a textarea, an editor with its own cursor cell). kit.Field
// re-wraps its value, which trims the trailing cursor cell.
func fieldBox(t *theme.Theme, w int, label, help string, lines []string, rows int, focused bool) string {
	lab := paint(t.Text.Muted, false, label)
	bar := paint(t.Surface.Sunken, false, t.Glyphs.BarThin)
	if focused {
		lab = paint(t.Text.Strong, true, label)
		bar = paint(t.Accent.Base, false, t.Glyphs.BarThin)
	}
	helpText := paint(t.Text.Faint, false, help)
	head := kit.Truncate(lab, max(w-kit.Width(helpText)-1, 0), "…")
	out := []string{kit.Fit(kit.PadRight(head, w-kit.Width(helpText))+helpText, w)}
	for i := range max(rows, 1) {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		out = append(out, kit.Backfill(kit.Fit(bar+" "+l, w), t.Surface.Sunken))
	}
	return strings.Join(out, "\n")
}
