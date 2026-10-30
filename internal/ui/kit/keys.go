package kit

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// FormatKeys turns a key spec into display text with the glyph set g.
// A spec is a space-separated sequence of chords; a chord joins modifiers and
// a key with "+": "ctrl+space p" → "⌃␣ p", "shift+tab" → "⇧tab",
// "enter" → "↵". Unknown names are shown as written. Modifiers: ctrl, shift,
// alt (opt). Named keys: space, enter, esc, tab, backspace, up, down, left,
// right.
func FormatKeys(g theme.Glyphs, spec string) string {
	chords := strings.Fields(spec)
	for i, chord := range chords {
		parts := strings.Split(chord, "+")
		var b strings.Builder
		for j, p := range parts {
			if j < len(parts)-1 {
				b.WriteString(modifierGlyph(g, p))
				continue
			}
			b.WriteString(keyGlyph(g, p))
		}
		chords[i] = b.String()
	}
	return strings.Join(chords, " ")
}

func modifierGlyph(g theme.Glyphs, m string) string {
	switch strings.ToLower(m) {
	case "ctrl", "control":
		return g.Ctrl
	case "shift":
		return g.Shift
	case "alt", "opt", "option", "meta":
		return g.Alt
	}
	return m + "+"
}

func keyGlyph(g theme.Glyphs, k string) string {
	switch strings.ToLower(k) {
	case "space":
		return g.Space
	case "enter", "return":
		return g.Enter
	case "esc", "escape":
		return g.Esc
	case "tab":
		return g.Tab
	case "backspace":
		return g.Backspace
	case "up":
		return g.ArrowUp
	case "down":
		return g.ArrowDown
	case "left":
		return g.ArrowLeft
	case "right":
		return g.ArrowRight
	}
	return k
}
