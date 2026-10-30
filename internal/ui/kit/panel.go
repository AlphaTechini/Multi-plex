package kit

import (
	"image/color"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// PanelOptions describes a Panel.
type PanelOptions struct {
	// Width and Height are the panel's outer size in cells, frame included.
	Width, Height int
	// Title is plain text set into the top edge ("╭─ Title ─────╮").
	Title string
	// Icon is an optional pre-rendered one-cell symbol before the title, for
	// example a StateDot.
	Icon string
	// Header replaces Title with pre-rendered text, for example a TabBar.
	Header string
	// Meta is plain text at the right end of the top edge, in muted text.
	Meta string
	// Footer is pre-rendered text set into the bottom edge, for example
	// KeyHints with width 0.
	Footer string
	// Focused draws the frame in the focus colour and the title in strong
	// text. Exactly one region on screen should be focused.
	Focused bool
	// Content is the body, one string per line. It is clipped and padded to
	// the inner size, so callers can pass more or fewer lines than fit.
	Content string
	// Bg is the panel surface. Nil means t.Surface.Base.
	Bg color.Color
	// Flush removes the one-cell horizontal padding inside the frame, for
	// content that draws edge to edge (agent terminal, diff lines).
	Flush bool
	// BorderColor overrides the frame colour (modals use a tone colour).
	BorderColor color.Color
}

// PanelInner returns the content size of a panel with the given options:
// what to render Content at so nothing is clipped.
func PanelInner(o PanelOptions) (w, h int) {
	pad := 1
	if o.Flush {
		pad = 0
	}
	return max(o.Width-2-2*pad, 0), max(o.Height-2, 0)
}

// Panel renders a titled region: a rounded, light frame with the title in the
// top edge and the content inside, exactly Width × Height cells. Panels are
// laid side by side, never nested; inside a panel, structure comes from
// spacing, rules and surfaces, not more frames.
func Panel(t *theme.Theme, o PanelOptions) string {
	bg := o.Bg
	if bg == nil {
		bg = t.Surface.Base
	}
	w, h := o.Width, o.Height
	if w < 2 || h < 2 {
		return Backfill(FitBlock("", w, h), bg)
	}
	g := t.Glyphs.Frame
	border := o.BorderColor
	if border == nil {
		border = t.Border.Default
		if o.Focused {
			border = t.Border.Focus
		}
	}
	edge := fg(border)
	inner := w - 2
	cw, ch := PanelInner(o)
	pad := spaces((inner - cw) / 2)
	vert := edge.paint(g.Vertical)

	var b strings.Builder
	b.Grow((h + 1) * (w + 2*len(vert) + 64))
	b.WriteString(edge.paint(g.TopLeft))
	b.WriteString(panelTop(t, o, inner, edge))
	b.WriteString(edge.paint(g.TopRight))

	// Every line is backfilled once, at the end: Backfill of the joined block
	// is byte-for-byte the join of per-line Backfills.
	var src []string
	if cw > 0 {
		src = Lines(o.Content)
	}
	for i := range ch {
		b.WriteByte('\n')
		b.WriteString(vert)
		if cw > 0 {
			line := ""
			if i < len(src) {
				line = src[i]
			}
			b.WriteString(pad)
			b.WriteString(Fit(line, cw))
			b.WriteString(pad)
		} else {
			b.WriteString(spaces(inner))
		}
		b.WriteString(vert)
	}

	bottom := edge.paint(repeat(g.Horizontal, inner))
	if o.Footer != "" && inner > 4 {
		f := Truncate(o.Footer, inner-4, "…")
		fill := inner - 3 - stringWidth(f)
		bottom = edge.paint(g.Horizontal) + " " + f + " " + edge.paint(repeat(g.Horizontal, fill))
	}
	b.WriteByte('\n')
	b.WriteString(edge.paint(g.BottomLeft))
	b.WriteString(bottom)
	b.WriteString(edge.paint(g.BottomRight))
	return Backfill(b.String(), bg)
}

// panelTop renders the top edge between the corners: exactly inner cells.
func panelTop(t *theme.Theme, o PanelOptions, inner int, edge ink) string {
	h := t.Glyphs.Frame.Horizontal
	title := o.Header
	if title == "" && o.Title != "" {
		st := ink{fg: t.Text.Secondary}
		if o.Focused {
			st = ink{fg: t.Text.Strong, bold: true}
		}
		title = st.paint(o.Title)
		if o.Icon != "" {
			title = o.Icon + " " + title
		}
	}
	if title == "" || inner < 5 {
		return edge.paint(repeat(h, inner))
	}
	meta := ""
	if o.Meta != "" {
		meta = fg(t.Text.Muted).paint(o.Meta)
	}
	// Layout: "─ " title " " fill " " meta " ─"
	tw := stringWidth(title)
	mw := stringWidth(meta)
	if meta != "" && inner-6-mw < min(tw, 8) {
		meta, mw = "", 0
	}
	room := inner - 3
	if meta != "" {
		room -= mw + 3
	}
	title = Truncate(title, room, "…")
	tw = stringWidth(title)
	fill := inner - 3 - tw
	right := ""
	if meta != "" {
		fill -= mw + 3
		right = " " + meta + " " + edge.paint(h)
	}
	return edge.paint(h) + " " + title + " " + edge.paint(repeat(h, fill)) + right
}
