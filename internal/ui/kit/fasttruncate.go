package kit

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// truncate is ansi.Truncate(s, length, tail) with a fast path. For strings
// made only of printable ASCII, CSI/OSC sequences and standalone runes (see
// stringWidth) it walks the string once, mirroring ansi.Truncate's rules
// exactly: escape sequences are always kept, printable text stops at the
// limit and the tail is written where the first cell that does not fit
// would start. Other strings go to ansi.Truncate.
func truncate(s string, length int, tail string) string {
	sw, ok := fastWidth(s)
	if !ok {
		return ansi.Truncate(s, length, tail)
	}
	if sw <= length {
		return s
	}
	length -= stringWidth(tail)
	if length < 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s) + len(tail))
	cur := 0
	ignoring := false
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == 0x1B:
			n := escapeLen(s[i:])
			b.WriteString(s[i : i+n])
			i += n
		case c < 0x80:
			if !ignoring && cur >= length {
				ignoring = true
				b.WriteString(tail)
			}
			if !ignoring {
				cur++
				b.WriteByte(c)
			}
			i++
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			cur += runeWidth(r)
			if !ignoring {
				if cur > length {
					ignoring = true
					b.WriteString(tail)
				} else {
					b.WriteString(s[i : i+size])
				}
			}
			i += size
		}
	}
	return b.String()
}

// fastWidth returns the width of s and true when s qualifies for the fast
// paths: printable ASCII, CSI and OSC sequences and standalone runes only
// (no control characters).
func fastWidth(s string) (int, bool) {
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
				return 0, false
			}
			i += n
		case c < 0x80:
			return 0, false
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			rw := runeWidth(r)
			if rw < 0 || (r == utf8.RuneError && size == 1) {
				return 0, false
			}
			w += rw
			i += size
		}
	}
	return w, true
}
