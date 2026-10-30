package kit

// Truncate shortens a single line to at most w cells, ending it with tail
// (for example "…") when it had to cut. It never splits an escape sequence or
// a wide character, and the result is never wider than w, even when tail is
// wider than w (the tail is then dropped). It returns "" for w <= 0.
func Truncate(s string, w int, tail string) string {
	if w <= 0 {
		return ""
	}
	if stringWidth(s) <= w {
		return s
	}
	if stringWidth(tail) > w {
		tail = ""
	}
	return truncate(s, w, tail)
}
