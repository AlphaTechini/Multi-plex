package main

import (
	"image/color"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// paint is a small local painter for fake content (agent transcript, code).
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

// paintBg paints text with a foreground and background.
func paintBg(c, bg color.Color, bold bool, text string) string {
	var st ansi.Style
	if bold {
		st = st.Bold()
	}
	if c != nil {
		st = st.ForegroundColor(c)
	}
	if bg != nil {
		st = st.BackgroundColor(bg)
	}
	return st.Styled(text)
}

// easeOut is the theme's easing for preview animation inputs.
func easeOut(p float64) float64 { return theme.EaseOutCubic(p) }
