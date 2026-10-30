package codeview

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/syntax"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// freshLevel returns the strength of the new-lines highlight: 1 when the
// lines just arrived, falling to 0 over the theme's decay in its steps; any
// positive value (a static marker) while not animated.
func (m *model) freshLevel() float64 {
	if !m.fresh.on {
		return 0
	}
	if !m.th.Animated() {
		return 1
	}
	d := m.th.Motion.HunkDecay
	if d <= 0 {
		return 0
	}
	p := float64(m.clock.Sub(m.fresh.at)) / float64(d)
	if p >= 1 {
		return 0
	}
	steps := float64(max(m.th.Motion.HunkDecaySteps, 1))
	level := 1 - theme.EaseOutCubic(p)
	return float64(int(level*steps+0.999)) / steps
}

// sgrKey identifies a foreground/background pair by value.
type sgrKey struct{ fg, bg [4]uint32 }

func rgba(c color.Color) (k [4]uint32) {
	if c != nil {
		k[0], k[1], k[2], k[3] = c.RGBA()
	}
	return k
}

// sgr returns the escape sequence that sets fg on bg, cached: a code line is
// painted from a few dozen distinct pairs, and building each sequence is the
// most expensive part of drawing one.
func (m *model) sgr(fg, bg color.Color) string {
	k := sgrKey{rgba(fg), rgba(bg)}
	if s, ok := m.seqs[k]; ok {
		return s
	}
	var st ansi.Style
	if fg != nil {
		st = st.ForegroundColor(fg)
	}
	if bg != nil {
		st = st.BackgroundColor(bg)
	}
	s := st.String()
	if len(m.seqs) > 256 { // blended highlight colours are unbounded in principle
		clear(m.seqs)
	}
	m.seqs[k] = s
	return s
}

// codeLine appends line i (0-based) to b: marker, line number and the code on
// the line's surface, exactly nw+cw+4 cells (nw digits, cw cells of code).
func (m *model) codeLine(b *strings.Builder, i, nw, cw int, hl float64) {
	th := m.th
	bg := th.Surface.Base
	numC := th.Diff.LineNumber
	marker, markerC := " ", th.Text.Faint
	cursor := i == m.cursor
	if cursor {
		bg = th.Surface.Hover
		numC = th.Text.Strong
		if m.focused {
			marker, markerC = th.Glyphs.Bar, th.Accent.Base
		}
	}
	if hl > 0 && i >= m.fresh.first && i <= m.fresh.last {
		if th.Animated() {
			bg = theme.Blend(bg, th.Diff.NewHunkBg, hl)
		} else if !cursor || !m.focused {
			marker, markerC = th.Glyphs.BarThin, th.Accent.Base
		}
	}
	b.WriteString(m.sgr(markerC, th.Surface.Base))
	b.WriteString(marker)
	b.WriteString(m.sgr(numC, bg))
	for n := len(itoa(i + 1)); n < nw; n++ {
		b.WriteByte(' ')
	}
	b.WriteString(itoa(i + 1))
	b.WriteString("  ")
	col := 0
	if toks := m.src.Tokens(i); toks != nil {
		for _, tk := range toks {
			if col >= cw {
				break
			}
			m.codeToken(b, tk.Text, syntax.Color(th, tk.Kind, tk.Text), bg, &col, cw)
		}
	} else {
		text := m.src.Text(i)
		if len(text) > 4*cw+64 {
			text = text[:4*cw+64]
		}
		m.codeToken(b, text, th.Syntax.Plain, bg, &col, cw)
	}
	b.WriteString(m.sgr(nil, bg))
	for n := cw - col + 1; n > 0; n-- { // the rest of the column and one cell of right padding
		b.WriteByte(' ')
	}
	b.WriteString("\x1b[m")
}

// codeToken appends one token, expanded and cut to the cells left in the code
// column (ending in an ellipsis when it had to be cut).
func (m *model) codeToken(b *strings.Builder, text string, fg, bg color.Color, col *int, cw int) {
	start := *col
	txt := codeText(text, col)
	if txt == "" {
		return
	}
	b.WriteString(m.sgr(fg, bg))
	if *col <= cw {
		b.WriteString(txt)
		return
	}
	b.WriteString(ansi.Truncate(txt, cw-start, m.th.Glyphs.Ellipsis))
	*col = cw
}
