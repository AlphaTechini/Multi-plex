package kit

import "github.com/alphatechini/multi-plex/internal/ui/theme"

// Keycap renders a key spec as a small key cap: the key on the Control surface
// with one cell of padding each side (" ⌃␣ p "). Use it where keys are the
// main content (empty state, help), and KeyHint in dense rows.
func Keycap(t *theme.Theme, keys string) string {
	return ink{fg: t.Text.Strong, bg: t.Surface.Control, bold: true}.paint(" " + FormatKeys(t.Glyphs, keys) + " ")
}
