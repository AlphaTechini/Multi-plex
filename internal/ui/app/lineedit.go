package app

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

// lineEdit is a minimal single-line text editor: enough for the palette
// query, the base branch, the deny message and the typed code. It has no
// goroutines or timers (so no idle CPU), and draws its cursor itself.
type lineEdit struct {
	text []rune
	pos  int
}

func newLineEdit(s string) lineEdit {
	r := []rune(s)
	return lineEdit{text: r, pos: len(r)}
}

// String returns the text.
func (e *lineEdit) String() string { return string(e.text) }

// Set replaces the text and moves the cursor to the end.
func (e *lineEdit) Set(s string) {
	e.text = []rune(s)
	e.pos = len(e.text)
}

// Insert adds text at the cursor, dropping newlines.
func (e *lineEdit) Insert(s string) {
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(s)
	r := []rune(s)
	out := make([]rune, 0, len(e.text)+len(r))
	out = append(out, e.text[:e.pos]...)
	out = append(out, r...)
	out = append(out, e.text[e.pos:]...)
	e.text = out
	e.pos += len(r)
}

// Key applies a key press and reports whether it edited or moved.
func (e *lineEdit) Key(k tea.KeyPressMsg) bool {
	switch k.String() {
	case "backspace", "ctrl+h":
		if e.pos > 0 {
			e.text = append(e.text[:e.pos-1], e.text[e.pos:]...)
			e.pos--
		}
	case "delete", "ctrl+d":
		if e.pos < len(e.text) {
			e.text = append(e.text[:e.pos], e.text[e.pos+1:]...)
		}
	case "left", "ctrl+b":
		e.pos = max(e.pos-1, 0)
	case "right", "ctrl+f":
		e.pos = min(e.pos+1, len(e.text))
	case "home", "ctrl+a":
		e.pos = 0
	case "end", "ctrl+e":
		e.pos = len(e.text)
	case "ctrl+u":
		e.text = append([]rune(nil), e.text[e.pos:]...)
		e.pos = 0
	case "ctrl+k":
		e.text = e.text[:e.pos]
	case "ctrl+w", "alt+backspace":
		i := e.pos
		for i > 0 && unicode.IsSpace(e.text[i-1]) {
			i--
		}
		for i > 0 && !unicode.IsSpace(e.text[i-1]) {
			i--
		}
		e.text = append(e.text[:i], e.text[e.pos:]...)
		e.pos = i
	default:
		if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			e.Insert(k.Text)
			return true
		}
		return false
	}
	return true
}

// Render draws the text with a block cursor cell (an accent-coloured space or
// the character under it), padded or clipped to width w. The cursor stays in
// view by scrolling the text horizontally.
func (e *lineEdit) Render(w int, text, cursorBg func(string) string) string {
	if w <= 0 {
		return ""
	}
	start := 0
	if e.pos >= w {
		start = e.pos - w + 1
	}
	var b strings.Builder
	n := 0
	for i := start; i <= len(e.text) && n < w; i++ {
		if i == e.pos {
			ch := " "
			if i < len(e.text) {
				ch = string(e.text[i])
			}
			b.WriteString(cursorBg(ch))
			n++
			continue
		}
		if i < len(e.text) {
			b.WriteString(text(string(e.text[i])))
			n++
		}
	}
	return b.String()
}
