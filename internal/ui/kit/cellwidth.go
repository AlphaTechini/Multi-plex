package kit

import (
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// stringWidth returns exactly what ansi.StringWidth returns for s, much
// faster for the strings the UI draws: printable ASCII, CSI and OSC escape
// sequences, and non-ASCII runes that always form a grapheme cluster on their
// own (box drawing, symbols, Latin, Greek, Cyrillic, CJK). Anything else
// (combining marks, joiners, variation selectors, complex scripts, emoji
// outside the BMP, malformed sequences) falls back to ansi.StringWidth for the
// whole string, so the result never differs.
func stringWidth(s string) int {
	w := 0
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c >= 0x20 && c < 0x7F:
			w++
			i++
		case c == 0x1B:
			n := escapeLen(s[i:])
			if n == 0 {
				return ansi.StringWidth(s)
			}
			i += n
		case c < 0x80:
			i++ // C0 controls and DEL take no cells
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			rw := runeWidth(r)
			if rw < 0 || (r == utf8.RuneError && size == 1) {
				return ansi.StringWidth(s)
			}
			w += rw
			i += size
		}
	}
	return w
}

// escapeLen returns the length of the CSI ("\x1b[...") or OSC ("\x1b]...")
// sequence at the start of s, or 0 for anything else, including unterminated
// sequences.
func escapeLen(s string) int {
	if len(s) < 2 {
		return 0
	}
	switch s[1] {
	case '[':
		for j := 2; j < len(s); j++ {
			b := s[j]
			switch {
			case b >= 0x40 && b <= 0x7E:
				return j + 1
			case b >= 0x20 && b <= 0x3F:
			default:
				return 0
			}
		}
	case ']':
		for j := 2; j < len(s); j++ {
			switch s[j] {
			case 0x07:
				return j + 1
			case 0x1B:
				if j+1 < len(s) && s[j+1] == '\\' {
					return j + 2
				}
				return 0
			}
		}
	}
	return 0
}

// widthPage caches the widths of 256 consecutive BMP runes; -1 marks a rune
// that may join a grapheme cluster with its neighbours.
type widthPage struct {
	once sync.Once
	w    [256]int8
}

var widthPages [256]widthPage

// runeWidth returns the cell width of a rune that always forms its own
// grapheme cluster, or -1.
func runeWidth(r rune) int {
	if r < 0 || r > 0xFFFF {
		return -1
	}
	p := &widthPages[r>>8]
	p.once.Do(func() {
		base := rune(r>>8) << 8
		for k := range 256 {
			q := base + rune(k)
			if standalone(q) {
				p.w[k] = int8(ansi.StringWidth(string(q)))
			} else {
				p.w[k] = -1
			}
		}
	})
	return int(p.w[r&0xFF])
}

// standalone reports whether r is a rune that never joins a grapheme cluster
// with the runes around it, in the scripts and symbol blocks the UI uses.
func standalone(r rune) bool {
	inRange := (r >= 0xA0 && r < 0x0300) || (r >= 0x0370 && r < 0x0500) ||
		(r >= 0x2000 && r < 0x2C00) || (r >= 0x2E00 && r < 0x2E80) ||
		(r >= 0x3000 && r < 0xA000) || (r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r < 0xFB00) || (r >= 0xFF00 && r < 0xFFF0)
	if !inRange || r == 0xFF9E || r == 0xFF9F {
		return false
	}
	return !unicode.In(r, unicode.Mn, unicode.Me, unicode.Mc, unicode.Cf, unicode.Cc, unicode.Cs, unicode.Co)
}
