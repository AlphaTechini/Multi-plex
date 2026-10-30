package overview

import (
	"image/color"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// paint colours text with a foreground, optionally bold, ending with a reset.
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

// paintBg is paint with a background.
func paintBg(c, bg color.Color, bold bool, text string) string {
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
	if bg != nil {
		st = st.BackgroundColor(bg)
	}
	return st.Styled(text)
}

// clean makes agent-written text safe to draw: escape sequences are removed,
// other control characters become spaces, and with oneLine set newlines and
// tabs collapse to single spaces too. Agents are untrusted: nothing they
// write may reach the terminal as a control sequence.
func clean(s string, oneLine bool) string {
	if s == "" {
		return ""
	}
	if strings.ContainsRune(s, 0x1b) {
		s = ansi.Strip(s)
	}
	needs := false
	for _, r := range s {
		if unicode.IsControl(r) && (r != '\n' || oneLine) {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsControl(r) && (r != '\n' || oneLine) {
			if oneLine {
				if !space {
					b.WriteByte(' ')
				}
				space = true
				continue
			}
			r = ' '
		}
		space = r == ' '
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// wrapText word-wraps plain text to w cells, never returning an empty slice
// for non-empty text and never producing a line wider than w.
func wrapText(s string, w int) []string {
	if w <= 0 || s == "" {
		return nil
	}
	return strings.Split(ansi.Wrap(s, w, ""), "\n")
}
