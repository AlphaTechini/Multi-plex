package kit

// Fit returns a single line exactly w cells wide: truncated with "…" when
// longer, padded with spaces when shorter. It is the basic guarantee behind
// every primitive in this package. It returns "" for w <= 0.
func Fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := stringWidth(s)
	if sw <= w {
		return s + spaces(w-sw)
	}
	t := truncate(s, w, "…")
	return t + spaces(w-stringWidth(t))
}
