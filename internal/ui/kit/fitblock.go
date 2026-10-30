package kit

import "strings"

// FitBlock returns s as exactly h lines of exactly w cells: lines are fitted
// with Fit, extra lines are dropped and missing lines are blank. It returns
// "" when w or h is not positive.
func FitBlock(s string, w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	src := Lines(s)
	out := make([]string, h)
	for i := range h {
		line := ""
		if i < len(src) {
			line = src[i]
		}
		out[i] = Fit(line, w)
	}
	return strings.Join(out, "\n")
}
