package kit

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Backfill paints bg behind every cell of s that would otherwise show the
// terminal's default background. It re-applies bg after every reset inside s,
// so content rendered elsewhere (an agent's terminal, a lipgloss string) sits
// on the region's surface without holes. Each line ends with a reset. A nil bg
// returns s unchanged.
func Backfill(s string, bg color.Color) string {
	if bg == nil {
		return s
	}
	set := ink{bg: bg}.seq()
	var b strings.Builder
	b.Grow(len(s) + (strings.Count(s, "\n")+1)*(len(set)+len(ansi.ResetStyle)) + 4*len(set))
	b.WriteString(set)
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == '\n':
			b.WriteString(ansi.ResetStyle)
			b.WriteByte('\n')
			b.WriteString(set)
			i++
		case c == 0x1B && i+1 < len(s) && s[i+1] == '[':
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7E) && s[j] != '\n' {
				j++
			}
			if j >= len(s) || s[j] == '\n' {
				// Unterminated sequence: copy it as it is.
				b.WriteString(s[i:j])
				i = j
				continue
			}
			b.WriteString(s[i : j+1])
			if s[j] == 'm' && resetsBackground(s[i+2:j]) {
				b.WriteString(set)
			}
			i = j + 1
		default:
			k := i + 1
			for k < len(s) && s[k] != '\n' && s[k] != 0x1B {
				k++
			}
			b.WriteString(s[i:k])
			i = k
		}
	}
	b.WriteString(ansi.ResetStyle)
	return b.String()
}

// resetsBackground reports whether an SGR parameter string resets the
// background (full reset or 49). It scans in place, skipping the arguments of
// extended colours (38/48/58;5;n and 38/48/58;2;r;g;b).
func resetsBackground(p string) bool {
	if p == "" {
		return true
	}
	skip := 0
	for {
		k := strings.IndexByte(p, ';')
		part, more := p, k >= 0
		if more {
			part = p[:k]
		}
		if skip > 0 {
			skip--
		} else {
			switch part {
			case "0", "", "49":
				return true
			case "38", "48", "58":
				if more {
					next := p[k+1:]
					if j := strings.IndexByte(next, ';'); j >= 0 {
						next = next[:j]
					}
					switch next {
					case "5":
						skip = 2
					case "2":
						skip = 4
					}
				}
			}
		}
		if !more {
			return false
		}
		p = p[k+1:]
	}
}
