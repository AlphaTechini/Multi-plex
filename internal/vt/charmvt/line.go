package charmvt

import (
	"unicode"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Line returns row y as text with SGR styling, exactly cols cells wide.
//
// A wide character occupies two columns and its continuation cell is never
// emitted. Cell contents are checked against ansi.StringWidth (the measure
// clients use), and control characters are replaced by spaces, so the width
// invariant holds whatever the program printed. The row ends with a reset if
// any style is active. Hyperlinks (OSC 8) are not carried over.
func (em *emulator) Line(y int) string {
	em.mu.Lock()
	defer em.mu.Unlock()
	if y < 0 || y >= em.rows || em.closed {
		return ""
	}
	b := em.line[:0]
	var cur uv.Style
	styled := false // cur is non-default and active in the output
	blank := true   // nothing but default-style spaces so far
	for x := 0; x < em.cols; {
		c := em.e.CellAt(x, y)
		if c == nil || c.Width < 1 {
			// Missing cell, or an orphaned continuation cell: a blank.
			if styled {
				b = append(b, "\x1b[m"...)
				cur, styled = uv.Style{}, false
			}
			b = append(b, ' ')
			x++
			continue
		}
		w := min(c.Width, 2)
		if x+w > em.cols {
			w = 1 // a wide character cannot straddle the right edge
		}
		if !c.Style.Equal(&cur) {
			cur = c.Style
			b = appendSGR(b, &cur)
			styled = cur != (uv.Style{})
			blank = blank && !styled
		}
		if c.Content == "" || c.Content == " " {
			b = appendSpaces(b, w)
		} else {
			b = appendCell(b, c.Content, w)
			blank = false
		}
		x += w
	}
	if styled {
		b = append(b, "\x1b[m"...)
	}
	em.line = b
	if blank && len(b) == em.cols {
		return em.blank
	}
	return string(b)
}

func appendSpaces(b []byte, n int) []byte {
	for ; n > 0; n-- {
		b = append(b, ' ')
	}
	return b
}

// appendCell appends content padded or replaced so that it is exactly w
// columns wide.
func appendCell(b []byte, content string, w int) []byte {
	if len(content) == 1 {
		ch := content[0]
		if ch < 0x20 || ch == 0x7f {
			return appendSpaces(b, w)
		}
		b = append(b, ch)
		return appendSpaces(b, w-1)
	}
	if !safeText(content) {
		return appendSpaces(b, w)
	}
	got := ansi.StringWidth(content)
	switch {
	case got == w:
		return append(b, content...)
	case got < w:
		b = append(b, content...)
		return appendSpaces(b, w-got)
	default:
		return appendSpaces(b, w)
	}
}

// safeText reports whether s is valid UTF-8 without control characters and
// can stand alone as one cell: its first rune must start a grapheme cluster.
// Clients measure a row as a whole, so a cell beginning with a combining mark,
// zero-width joiner or variation selector would merge with the previous cell
// and shift every column after it. A lone regional indicator is refused for
// the same reason (two of them in neighbouring cells would form a flag).
func safeText(s string) bool {
	first := true
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n <= 1 {
			return false
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return false
		}
		if first && (extendsCluster(r) || (isRegionalIndicator(r) && n == len(s))) {
			return false
		}
		first = false
		i += n
	}
	return true
}

func isRegionalIndicator(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }

// extendsCluster reports whether r attaches to the character before it.
func extendsCluster(r rune) bool {
	switch {
	case r == 0x200d, r >= 0xfe00 && r <= 0xfe0f, r >= 0xe0100 && r <= 0xe01ef,
		r >= 0xe0020 && r <= 0xe007f, r >= 0x1f3fb && r <= 0x1f3ff:
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc)
}
