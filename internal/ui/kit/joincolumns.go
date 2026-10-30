package kit

import "strings"

// JoinColumns places blocks side by side, top-aligned. Each block is padded
// to its own widest line and to the tallest block's height, so the result is
// a rectangle. Build blocks with exact sizes (FitBlock, Panel) to control the
// padding colour.
func JoinColumns(blocks ...string) string {
	if len(blocks) == 0 {
		return ""
	}
	cols := make([][]string, len(blocks))
	widths := make([]int, len(blocks))
	height := 0
	for i, b := range blocks {
		cols[i] = Lines(b)
		widths[i] = Width(b)
		height = max(height, len(cols[i]))
	}
	out := make([]string, height)
	var sb strings.Builder
	for r := range height {
		sb.Reset()
		for i, col := range cols {
			line := ""
			if r < len(col) {
				line = col[r]
			}
			sb.WriteString(PadRight(line, widths[i]))
		}
		out[r] = sb.String()
	}
	return strings.Join(out, "\n")
}
