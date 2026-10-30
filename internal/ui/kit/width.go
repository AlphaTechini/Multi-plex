package kit

// Width returns the number of terminal cells s occupies. Escape sequences take
// no cells; wide characters (CJK, most emoji) take two. For multi-line
// strings it is the width of the widest line.
func Width(s string) int {
	w := 0
	for line := range splitLines(s) {
		w = max(w, stringWidth(line))
	}
	return w
}
