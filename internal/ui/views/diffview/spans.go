package diffview

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// codeSpans builds the coloured spans of one diff line: syntax tokens split
// at the word-level segments of the engine. A line without segments (an
// unpaired addition or deletion, or a context line) gets no word emphasis:
// the line background already marks it as wholly changed.
func (m *Model) codeSpans(ft *fileTokens, ln *diff.Line) []kit.Span {
	var segs []diff.Segment
	if ln.Kind != diff.Context && len(ln.Segments) > 0 {
		segs = ln.Segments
		if len(segs) == 1 && segs[0].Changed && segs[0].Start == 0 && segs[0].End >= len(ln.Text) {
			segs = nil
		}
	}
	return buildSpans(m.t, ln.Text, ft.lookup(ln), segs, m.hx)
}

// buildSpans merges tokens and changed-word segments into spans. Tabs are
// expanded to tab stops, control characters are made harmless and the first
// hx cells are skipped (sideways scroll).
func buildSpans(t *theme.Theme, text string, toks []highlight.Token, segs []diff.Segment, hx int) []kit.Span {
	if text == "" {
		return nil
	}
	if len(toks) == 0 {
		toks = []highlight.Token{{Text: text}}
	}
	spans := make([]kit.Span, 0, len(toks)+len(segs))
	col, off, si := 0, 0, 0
	for _, tk := range toks {
		a, b := off, off+len(tk.Text)
		off = b
		c := syntaxStyle(t, tk)
		for a < b {
			emph, end := false, b
			if segs != nil {
				for si < len(segs) && segs[si].End <= a {
					si++
				}
				if si < len(segs) {
					if s := segs[si]; s.Start > a {
						end = min(b, s.Start)
					} else {
						emph, end = s.Changed, min(b, s.End)
					}
				}
			}
			piece := cleanPiece(text[a:end], &col, hx)
			if piece != "" {
				if n := len(spans); n > 0 && spans[n-1].Fg == c && spans[n-1].Emph == emph {
					spans[n-1].Text += piece
				} else {
					spans = append(spans, kit.Span{Text: piece, Fg: c, Emph: emph})
				}
			}
			a = end
		}
	}
	return spans
}

// cleanPiece expands tabs from display column *col, replaces control
// characters, advances *col and drops the part left of hx.
func cleanPiece(p string, col *int, hx int) string {
	plain := true
	for i := 0; i < len(p); i++ {
		if c := p[i]; c < 0x20 || c == 0x7f || c >= 0x80 {
			plain = false
			break
		}
	}
	if plain {
		w := len(p)
		start := *col
		*col += w
		if *col <= hx {
			return ""
		}
		if start < hx {
			return p[hx-start:]
		}
		return p
	}
	var b strings.Builder
	start := *col
	c := *col
	for _, r := range p {
		switch {
		case r == '\t':
			n := tabWidth - c%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			c += n
		case r < 0x20 || r == 0x7f:
			b.WriteRune('·')
			c++
		default:
			b.WriteRune(r)
			c += ansi.StringWidth(string(r))
		}
	}
	*col = c
	s := b.String()
	if c <= hx {
		return ""
	}
	if start < hx {
		return ansi.TruncateLeft(s, hx-start, "")
	}
	return s
}
