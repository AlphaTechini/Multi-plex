package term

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// LastLines returns the last n non-empty screen lines as plain text, oldest
// first, with trailing spaces removed. The daemon uses it as the Overview
// fallback for "what the agent last said" when no report_progress arrived.
func (h *Host) LastLines(n int) []string {
	if n <= 0 {
		return nil
	}
	_, rows := h.emu.Size()
	var out []string
	for y := rows - 1; y >= 0 && len(out) < n; y-- {
		line := strings.TrimRight(ansi.Strip(h.emu.Line(y)), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
