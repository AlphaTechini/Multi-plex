package codeview

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

// tabWidth is the width of a tab stop in the code column.
const tabWidth = 4

// codeText makes one token of file content drawable: tabs expand to the next
// tab stop, other control characters become U+FFFD (a file may hold escape
// sequences, and none may reach the terminal), and col, the column the token
// starts at, advances by the token's width.
func codeText(s string, col *int) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c >= 0x7f {
			ascii = false
			break
		}
	}
	if ascii {
		*col += len(s)
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			n := tabWidth - *col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			*col += n
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
			b.WriteRune('�')
			*col++
		default:
			b.WriteRune(r)
			*col += ansi.StringWidth(string(r))
		}
	}
	return b.String()
}
