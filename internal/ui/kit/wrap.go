package kit

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Wrap word-wraps s (plain or styled) to lines of at most w cells, breaking
// words longer than w, and returns the lines. Each line starts with the
// styles still active from the lines before it, so every line can be fitted
// and backfilled on its own. Use it instead of truncating text the user must
// read in full, such as the command in an approval prompt.
func Wrap(s string, w int) []string {
	if w <= 0 {
		return nil
	}
	out := Lines(ansi.Hardwrap(ansi.Wordwrap(s, w, ""), w, true))
	active := ""
	for i, l := range out {
		l = strings.TrimRight(l, " ")
		if i > 0 && active != "" {
			l = active + l
		}
		rewriteSGR(l, func(p string) string {
			if p == "" || p == "0" {
				active = ""
			} else {
				active += "\x1b[" + p + "m"
			}
			return ""
		})
		out[i] = l
	}
	return out
}
