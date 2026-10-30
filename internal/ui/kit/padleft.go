package kit

// PadLeft prepends spaces to a single line until it is w cells wide
// (right-aligns it). A line already w cells or wider is returned unchanged.
func PadLeft(s string, w int) string {
	return spaces(w-stringWidth(s)) + s
}
