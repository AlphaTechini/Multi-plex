package kit

import (
	"image/color"
	"strings"
	"sync"

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

// seqCache memoises ink sequences: building one costs several allocations
// and the UI paints the same few dozen inks every frame.
var (
	seqMu    sync.RWMutex
	seqCache = map[ink]string{}
)

// seq returns the SGR sequence that sets the ink, or "" for an empty ink.
func (k ink) seq() string {
	if !k.cacheable() {
		return k.build()
	}
	seqMu.RLock()
	s, ok := seqCache[k]
	seqMu.RUnlock()
	if ok {
		return s
	}
	s = k.build()
	seqMu.Lock()
	if len(seqCache) > 4096 { // themes are small; this only guards against misuse
		clear(seqCache)
	}
	seqCache[k] = s
	seqMu.Unlock()
	return s
}

// cacheable reports whether the ink's colours are of comparable types, so the
// ink can be a map key.
func (k ink) cacheable() bool { return comparableColor(k.fg) && comparableColor(k.bg) }

func comparableColor(c color.Color) bool {
	switch c.(type) {
	case nil, color.RGBA, color.NRGBA, ansi.IndexedColor, ansi.BasicColor:
		return true
	}
	return false
}

// build constructs the SGR sequence for the ink.
func (k ink) build() string {
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

// spaceRun backs spaces for the common short runs, without allocating.
var spaceRun = strings.Repeat(" ", 512)

// spaces returns n spaces (none for n <= 0).
func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	if n <= len(spaceRun) {
		return spaceRun[:n]
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
