package diffview

import (
	"image/color"

	"github.com/charmbracelet/x/ansi"
)

// paint colours a short piece of text with theme colours (the kit's own
// painter is private). It is used for the rail and the separator only; code
// goes through kit.DiffLine.
func paint(fg color.Color, bold bool, text string) string {
	if text == "" {
		return ""
	}
	var st ansi.Style
	if bold {
		st = st.Bold()
	}
	if fg != nil {
		st = st.ForegroundColor(fg)
	}
	return st.Styled(text)
}
