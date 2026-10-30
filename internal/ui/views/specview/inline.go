package specview

import (
	"image/color"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// word is one unbreakable run of styled text and its width in cells.
type word struct {
	s string
	w int
}

// frag is a run of text in one inline style.
type frag struct {
	text string
	fg   color.Color
	bold bool
}

// earsWords are the EARS keywords of acceptance criteria (docs/09), drawn
// quiet so the requirement itself reads first.
var earsWords = map[string]bool{
	"WHEN": true, "IF": true, "THEN": true, "WHILE": true, "WHERE": true,
	"THE": true, "SYSTEM": true, "SHALL": true, "NOT": true,
}

// inline turns one line of markdown-lite text into words: `code` spans in the
// variable colour, **bold** and __bold__ in strong text, [links](url) as
// their text in the accent colour, and emphasis markers dropped. base is the
// colour of plain text. With ears set, EARS keywords are muted and bold.
func inline(t *theme.Theme, s string, base color.Color, bold, ears bool) []word {
	var frags []frag
	var cur strings.Builder
	flush := func(fg color.Color, b bool) {
		if cur.Len() > 0 {
			frags = append(frags, frag{text: cur.String(), fg: fg, bold: b})
			cur.Reset()
		}
	}
	rs := []rune(s)
	isBold := bold
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '`':
			j := indexRune(rs, '`', i+1)
			if j < 0 {
				cur.WriteRune(r)
				continue
			}
			flush(base, isBold)
			frags = append(frags, frag{text: string(rs[i+1 : j]), fg: t.Syntax.Variable})
			i = j
		case (r == '*' || r == '_') && i+1 < len(rs) && rs[i+1] == r:
			// ** or __ toggles bold; a lone _ inside a word is text.
			if r == '_' && i > 0 && isWordRune(rs[i-1]) && !isBold {
				cur.WriteRune(r)
				continue
			}
			flush(base, isBold)
			isBold = !isBold
			i++
		case r == '*' && (i+1 < len(rs) && rs[i+1] != ' '):
			// single * emphasis: drop the marker
		case r == '*' && i > 0 && rs[i-1] != ' ':
			// closing single *
		case r == '[':
			close := indexRune(rs, ']', i+1)
			if close > 0 && close+1 < len(rs) && rs[close+1] == '(' {
				if end := indexRune(rs, ')', close+2); end > 0 {
					flush(base, isBold)
					frags = append(frags, frag{text: string(rs[i+1 : close]), fg: t.Accent.Base})
					i = end
					continue
				}
			}
			cur.WriteRune(r)
		case r == '\\' && i+1 < len(rs) && strings.ContainsRune("\\`*_{}[]()#+-.!|<>", rs[i+1]):
			cur.WriteRune(rs[i+1])
			i++
		default:
			cur.WriteRune(r)
		}
	}
	flush(base, isBold)
	return toWords(t, frags, ears)
}

// toWords splits fragments on spaces into words, painting each fragment.
func toWords(t *theme.Theme, frags []frag, ears bool) []word {
	var words []word
	var b strings.Builder
	w := 0
	flush := func() {
		if w > 0 {
			words = append(words, word{b.String(), w})
		}
		b.Reset()
		w = 0
	}
	for _, f := range frags {
		parts := strings.Split(f.text, " ")
		for pi, p := range parts {
			if pi > 0 {
				flush()
			}
			if p == "" {
				continue
			}
			fg, bold := f.fg, f.bold
			if ears && f.fg == nil && earsWords[p] {
				fg, bold = t.Text.Muted, true
			}
			b.WriteString(paint(fg, bold, p))
			w += ansi.StringWidth(p)
		}
	}
	flush()
	return words
}

// wrapWords lays words out in lines of at most width cells. A word wider
// than the line is cut with an ellipsis.
func wrapWords(words []word, width int, ellipsis string) []string {
	if width < 1 {
		return nil
	}
	var lines []string
	var b strings.Builder
	w := 0
	for _, wd := range words {
		if wd.w > width {
			if w > 0 {
				lines = append(lines, b.String())
				b.Reset()
				w = 0
			}
			lines = append(lines, ansi.Truncate(wd.s, width, ellipsis))
			continue
		}
		if w > 0 && w+1+wd.w > width {
			lines = append(lines, b.String())
			b.Reset()
			w = 0
		}
		if w > 0 {
			b.WriteByte(' ')
			w++
		}
		b.WriteString(wd.s)
		w += wd.w
	}
	if w > 0 {
		lines = append(lines, b.String())
	}
	return lines
}

func indexRune(rs []rune, r rune, from int) int {
	for i := from; i < len(rs); i++ {
		if rs[i] == r {
			return i
		}
	}
	return -1
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
