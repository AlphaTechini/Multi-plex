package kit

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// SessionItemOptions describes a SessionItem.
type SessionItemOptions struct {
	// Width is the exact width in cells. Widths below 8 render the compact
	// icon form used by the collapsed sidebar.
	Width int
	// Name is the session slug, for example "auth-refactor".
	Name string
	// State drives the dot, and the detail colour for loud states.
	State State
	// Frame is the spinner frame for StateWorking.
	Frame int
	// Detail is the second line: role and age ("implementer · 14m"), or what
	// the session needs ("needs approval").
	Detail string
	// Badge is optional right-aligned text, usually the unread card count.
	Badge string
	// BadgeTone colours the badge; the default is muted.
	BadgeTone Tone
	// Selected, Focused as in RowOptions.
	Selected, Focused bool
	// Unseen marks a session with news since it was last viewed (a done
	// session stays highlighted until viewed): the name is strong and bold.
	Unseen bool
}

// SessionItem renders a sidebar session entry as two rows: the state dot,
// name and badge, then the detail line indented under the name.
func SessionItem(t *theme.Theme, o SessionItemOptions) string {
	w := o.Width
	if w <= 0 {
		return "\n"
	}
	bg := rowBg(t, nil, o.Selected, o.Focused)
	marker := " "
	if o.Selected && o.Focused {
		marker = fg(t.Accent.Base).paint(t.Glyphs.Bar)
	}
	dot := StateDot(t, o.State, o.Frame)
	badge := ""
	if o.Badge != "" {
		tone := o.BadgeTone
		if tone == ToneDefault {
			tone = ToneMuted
		}
		badge = fg(ToneColor(t, tone)).paint(o.Badge)
	}
	if w < 8 {
		top := marker + " " + dot
		bottom := " " + " " + badge
		return row(top, w, bg) + "\n" + row(bottom, w, bg)
	}
	name := ink{fg: t.Text.Primary}
	if o.Selected || o.Unseen {
		name = ink{fg: t.Text.Strong, bold: o.Unseen}
	}
	detail := fg(t.Text.Muted)
	if o.State.Loud() {
		detail = fg(o.State.Color(t))
	}
	top := marker + " " + dot + " " + spread(name.paint(o.Name), badge, w-5) + " "
	bottom := marker + "   " + Fit(detail.paint(o.Detail), w-5) + " "
	return strings.Join([]string{Backfill(top, bg), Backfill(bottom, bg)}, "\n")
}
