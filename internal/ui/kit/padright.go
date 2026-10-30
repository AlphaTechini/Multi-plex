package kit

import "github.com/charmbracelet/x/ansi"

// PadRight appends spaces to a single line until it is w cells wide. A line
// already w cells or wider is returned unchanged; use Fit for an exact width.
func PadRight(s string, w int) string {
	return s + spaces(w-ansi.StringWidth(s))
}
