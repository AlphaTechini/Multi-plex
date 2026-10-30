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
	lines := Lines(s)
	for i, line := range lines {
		body := rewriteSGR(line, func(p string) string {
			seq := "\x1b[" + p + "m"
			if resetsBackground(p) {
				return seq + set
			}
			return seq
		})
		lines[i] = set + body + ansi.ResetStyle
	}
	return strings.Join(lines, "\n")
}

// resetsBackground reports whether an SGR parameter string resets the
// background (full reset or 49).
func resetsBackground(p string) bool {
	parts := sgrParams(p)
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "0", "":
			return true
		case "49":
			return true
		case "38", "48", "58":
			i += colorArgs(parts, i)
		}
	}
	return false
}
