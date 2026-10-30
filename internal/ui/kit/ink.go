package kit

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// ink is the internal painter: a foreground, background and attributes
// turned into one SGR sequence. It is faster and more predictable than a
// lipgloss style for single-line spans, which is all the kit draws.
type ink struct {
	fg, bg                  color.Color
	bold, italic, underline bool
	strike                  bool
}

// seq returns the SGR sequence that sets the ink, or "" for an empty ink.
func (k ink) seq() string {
	var s ansi.Style
	if k.bold {
		s = s.Bold()
	}
	if k.italic {
		s = s.Italic(true)
	}
	if k.underline {
		s = s.Underline(true)
	}
	if k.strike {
		s = s.Strikethrough(true)
	}
	if k.fg != nil {
		s = s.ForegroundColor(k.fg)
	}
	if k.bg != nil {
		s = s.BackgroundColor(k.bg)
	}
	if len(s) == 0 {
		return ""
	}
	return s.String()
}

// paint wraps text in the ink and a reset. Empty text stays empty. Text that
// is already styled keeps its own styles: the ink is re-applied after every
// reset inside it, so nesting a styled string in a painted one is safe.
func (k ink) paint(text string) string {
	if text == "" {
		return ""
	}
	seq := k.seq()
	if seq == "" {
		return text
	}
	if strings.Contains(text, "\x1b[") {
		text = rewriteSGR(text, func(p string) string {
			if p == "" || p == "0" {
				return "\x1b[" + p + "m" + seq
			}
			return "\x1b[" + p + "m"
		})
	}
	return seq + text + ansi.ResetStyle
}

// fg is shorthand for an ink with only a foreground.
func fg(c color.Color) ink { return ink{fg: c} }

// spaces returns n spaces (none for n <= 0).
func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

// repeat returns glyph repeated to fill exactly n cells. Glyphs are one cell.
func repeat(glyph string, n int) string {
	if n <= 0 || glyph == "" {
		return ""
	}
	out := make([]byte, 0, n*len(glyph))
	for range n {
		out = append(out, glyph...)
	}
	return string(out)
}
