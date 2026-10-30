package app

import (
	"image/color"

	"github.com/charmbracelet/x/ansi"
)

// paint styles text with a foreground colour and optional bold. The kit has
// no exported text painter; the shell needs one for the few runs it styles
// itself (always with theme colours).
func paint(c color.Color, bold bool, text string) string {
	if text == "" {
		return ""
	}
	var st ansi.Style
	if bold {
		st = st.Bold()
	}
	if c != nil {
		st = st.ForegroundColor(c)
	}
	return st.Styled(text)
}

// paintBg styles text with foreground and background colours.
func paintBg(fg, bg color.Color, bold bool, text string) string {
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
	if bg != nil {
		st = st.BackgroundColor(bg)
	}
	return st.Styled(text)
}
