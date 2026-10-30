package pane

import (
	"image/color"
	"strconv"

	"github.com/charmbracelet/x/ansi"
)

func paint(c color.Color, text string) string {
	if text == "" {
		return ""
	}
	return ansi.NewStyle().ForegroundColor(c).Styled(text)
}

func itoa(n int) string { return strconv.Itoa(n) }
