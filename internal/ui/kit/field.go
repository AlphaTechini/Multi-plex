package kit

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// FieldOptions describes a Field.
type FieldOptions struct {
	// Width is the exact width in cells.
	Width int
	// Label is shown above the box.
	Label string
	// Help is optional right-aligned text on the label row (faint).
	Help string
	// Value is the content: plain text (wrapped) or pre-rendered lines, for
	// example a bubbles textinput or textarea View.
	Value string
	// Placeholder is shown, faint, when Value is empty.
	Placeholder string
	// Rows is the box height in lines (default 1).
	Rows int
	// Focused draws the accent bar and a strong label.
	Focused bool
	// Cursor draws a block cursor after the value when focused (for static
	// mockups; live inputs draw their own cursor).
	Cursor bool
}

// FieldHeight returns how many lines Field renders: the label row plus Rows.
func FieldHeight(o FieldOptions) int { return 1 + max(o.Rows, 1) }

// Field renders a labelled input: a label row (transparent, so it sits on any
// surface) over an inset box on the sunken surface. The focused field has an
// accent bar on the box's left edge and a strong label; others are quiet.
func Field(t *theme.Theme, o FieldOptions) string {
	w := o.Width
	rows := max(o.Rows, 1)
	label := fg(t.Text.Muted)
	bar := fg(t.Surface.Sunken).paint(t.Glyphs.BarThin)
	if o.Focused {
		label = ink{fg: t.Text.Strong, bold: true}
		bar = fg(t.Accent.Base).paint(t.Glyphs.BarThin)
	}
	lines := []string{row(spread(label.paint(o.Label), fg(t.Text.Faint).paint(o.Help), w), w, nil)}
	cw := max(w-3, 0)
	var content []string
	if o.Value == "" {
		content = []string{fg(t.Text.Faint).paint(o.Placeholder)}
		if o.Focused && o.Cursor {
			content = []string{cursorCell(t) + fg(t.Text.Faint).paint(Truncate(o.Placeholder, cw-1, "…"))}
		}
	} else {
		for _, l := range wrap(o.Value, cw-1) {
			content = append(content, fg(t.Text.Primary).paint(l))
		}
		if o.Focused && o.Cursor && len(content) > 0 {
			content[len(content)-1] += cursorCell(t)
		}
	}
	for i := range rows {
		c := ""
		if i < len(content) {
			c = content[i]
		}
		lines = append(lines, row(bar+" "+c, w, t.Surface.Sunken))
	}
	return strings.Join(lines, "\n")
}

func cursorCell(t *theme.Theme) string {
	return ink{bg: t.Accent.Base}.paint(" ")
}
