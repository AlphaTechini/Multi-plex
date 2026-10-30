package kit

import "github.com/alphatechini/multi-plex/internal/ui/theme"

// Hint is one key hint: a key spec (see FormatKeys) and what it does.
type Hint struct {
	// Keys is the key spec, for example "ctrl+space p".
	Keys string
	// Label is the action, lower case, for example "palette".
	Label string
}

// KeyHint renders one hint as the key in strong text followed by the label in
// muted text: "⌃␣ p palette".
func KeyHint(t *theme.Theme, h Hint) string {
	keys := ink{fg: t.Text.Secondary, bold: true}.paint(FormatKeys(t.Glyphs, h.Keys))
	if h.Label == "" {
		return keys
	}
	return keys + " " + fg(t.Text.Muted).paint(h.Label)
}
