package kit

import "github.com/charmbracelet/x/ansi"

// TruncateLeft shortens a single line to at most w cells by cutting from the
// left and prefixing head (for example "…"), keeping the end visible. Use it
// for paths, where the file name matters most ("…/auth/session.go"). A wide
// character split by the cut is replaced by a space, so the result is never
// wider than w. It returns "" for w <= 0.
func TruncateLeft(s string, w int, head string) string {
	if w <= 0 {
		return ""
	}
	sw := ansi.StringWidth(s)
	if sw <= w {
		return s
	}
	hw := ansi.StringWidth(head)
	if hw > w {
		head, hw = "", 0
	}
	keep := w - hw
	return head + cutLeft(s, sw-keep, keep)
}

// cutLeft drops the first n cells of s and returns at most want cells. When n
// falls inside a wide character, that character is replaced by a space.
func cutLeft(s string, n, want int) string {
	if want <= 0 {
		return ""
	}
	rest := ansi.TruncateLeft(s, n, "")
	if ansi.StringWidth(rest) > want {
		rest = " " + ansi.TruncateLeft(s, n+1, "")
	}
	return rest
}
