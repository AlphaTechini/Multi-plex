package kit

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Composite draws the block fg over the block bg with fg's top-left corner at
// column x, row y, and returns the result. bg keeps its size: parts of fg
// outside bg are clipped, and negative x or y clip fg's left or top. Cells of
// bg under fg are replaced; the rest keep their styling. A wide character in
// bg cut by fg's edge becomes a space.
func Composite(bg, fg string, x, y int) string {
	base := Lines(bg)
	over := Lines(fg)
	if y < 0 {
		if -y >= len(over) {
			return bg
		}
		over = over[-y:]
		y = 0
	}
	for i, line := range over {
		row := y + i
		if row >= len(base) {
			break
		}
		base[row] = overlayLine(base[row], line, x)
	}
	return strings.Join(base, "\n")
}

// overlayLine puts top over line starting at column x.
func overlayLine(line, top string, x int) string {
	lw := stringWidth(line)
	if x < 0 {
		top = cutLeft(top, -x, stringWidth(top)+x)
		x = 0
	}
	if x >= lw {
		return line
	}
	top = Truncate(top, lw-x, "")
	tw := stringWidth(top)
	if tw == 0 {
		return line
	}
	left := PadRight(truncate(line, x, ""), x)
	right := ""
	if x+tw < lw {
		right = cutLeft(line, x+tw, lw-x-tw)
	}
	return left + ansi.ResetStyle + top + ansi.ResetStyle + right
}
