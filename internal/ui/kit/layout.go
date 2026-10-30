package kit

import (
	"image/color"

	"github.com/charmbracelet/x/ansi"
)

// row fits s to exactly w cells and paints bg behind it.
func row(s string, w int, bg color.Color) string {
	if w <= 0 {
		return ""
	}
	return Backfill(Fit(s, w), bg)
}

// spread places left and right at the two ends of a w-cell line. When they do
// not fit, left is truncated first, keeping at least one space between them;
// if right alone does not fit, it is dropped.
func spread(left, right string, w int) string {
	if w <= 0 {
		return ""
	}
	rw := stringWidth(right)
	if rw == 0 {
		return Fit(left, w)
	}
	if rw+1 > w {
		return Fit(left, w)
	}
	left = Truncate(left, w-rw-1, "…")
	return PadRight(left, w-rw) + right
}

// wrap word-wraps plain or styled text to w cells and returns the lines.
func wrap(s string, w int) []string {
	if w <= 0 {
		return nil
	}
	return Lines(ansi.Wrap(s, w, ""))
}

// clampLines returns exactly n lines from src: extra lines are dropped (the
// last kept line gets an ellipsis) and missing lines are empty.
func clampLines(src []string, n, w int) []string {
	out := make([]string, n)
	for i := range n {
		if i < len(src) {
			out[i] = src[i]
		}
	}
	if len(src) > n && n > 0 {
		last := Truncate(out[n-1], w-1, "")
		out[n-1] = last + "…"
	}
	return out
}
