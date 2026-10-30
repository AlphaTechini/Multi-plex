package kit

import (
	"image/color"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// RowOptions describes a Row.
type RowOptions struct {
	// Width is the exact width in cells.
	Width int
	// Left is the main text (may be pre-styled).
	Left string
	// Right is right-aligned text (may be pre-styled), kept when Left is cut.
	Right string
	// Selected marks the row under the cursor.
	Selected bool
	// Focused tells whether the list holding the row has focus. A selected
	// row in a focused list gets the selection surface and an accent bar; in
	// an unfocused list it gets the quieter hover surface.
	Focused bool
	// Bg is the unselected surface; nil means t.Surface.Base.
	Bg color.Color
}

// Row renders one selectable list row: a one-cell marker column (the accent
// bar when selected and focused), the left text, and the right text with one
// cell of right padding. It is the building block of the sidebar, file rail
// and command palette lists.
func Row(t *theme.Theme, o RowOptions) string {
	if o.Width <= 0 {
		return ""
	}
	bg := rowBg(t, o.Bg, o.Selected, o.Focused)
	marker := " "
	if o.Selected && o.Focused {
		marker = fg(t.Accent.Base).paint(t.Glyphs.Bar)
	}
	if o.Width < 3 {
		return row(marker, o.Width, bg)
	}
	return Backfill(marker+spread(o.Left, o.Right, o.Width-2)+" ", bg)
}

// rowBg picks the row surface for a selection state.
func rowBg(t *theme.Theme, base color.Color, selected, focused bool) color.Color {
	switch {
	case selected && focused:
		return t.Surface.Selected
	case selected:
		return t.Surface.Hover
	case base != nil:
		return base
	default:
		return t.Surface.Base
	}
}
