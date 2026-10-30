package kit

import (
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// DividerOptions describes a Divider.
type DividerOptions struct {
	// Width is the exact width in cells.
	Width int
	// Label is optional plain text set into the rule.
	Label string
	// Align places the label (left labels keep a two-cell lead-in).
	Align Align
	// Glyph is the rule character; empty means t.Glyphs.Rule.
	Glyph string
	// Tone colours the label; the rule itself uses the subtle border colour.
	Tone Tone
}

// Divider renders a horizontal rule with an optional label:
// "── Label ─────────" (left) or "──── Label ────" (centre).
func Divider(t *theme.Theme, o DividerOptions) string {
	w := o.Width
	if w <= 0 {
		return ""
	}
	glyph := o.Glyph
	if glyph == "" {
		glyph = t.Glyphs.Rule
	}
	rule := fg(t.Border.Default)
	if o.Label == "" || w < 6 {
		return rule.paint(repeat(glyph, w))
	}
	tone := o.Tone
	if tone == ToneDefault {
		tone = ToneMuted
	}
	label := " " + Truncate(o.Label, w-6, "…") + " "
	lw := stringWidth(label)
	var left int
	switch o.Align {
	case AlignCenter:
		left = (w - lw) / 2
	case AlignRight:
		left = w - lw - 2
	default:
		left = 2
	}
	return rule.paint(repeat(glyph, left)) + fg(ToneColor(t, tone)).paint(label) + rule.paint(repeat(glyph, w-left-lw))
}
