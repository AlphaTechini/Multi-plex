package kit

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// ToastOptions describes a Toast.
type ToastOptions struct {
	// Width is the exact width in cells (about 44 is typical).
	Width int
	// Tone colours the bar and icon: ToneSuccess, ToneInfo, ToneWarning,
	// ToneDanger.
	Tone Tone
	// Title is the one-line message.
	Title string
	// Body is an optional second line (muted), truncated to one line.
	Body string
	// Hints are optional actions ("u undo").
	Hints []Hint
}

// Toast renders a transient notice on the overlay surface: a tone bar on the
// left, an icon, the title and an optional detail line, with one blank row of
// padding above and below. Place it with Composite at the bottom-right of the
// main view, above the status bar.
func Toast(t *theme.Theme, o ToastOptions) string {
	w := o.Width
	bg := t.Surface.Overlay
	tone := o.Tone
	if tone == ToneDefault {
		tone = ToneInfo
	}
	c := ToneColor(t, tone)
	bar := fg(c).paint(t.Glyphs.Bar)
	icon := fg(c).paint(toastIcon(t, tone))
	cw := max(w-6, 0)
	lines := []string{
		row(bar, w, bg),
		row(bar+" "+icon+"  "+ink{fg: t.Text.Strong, bold: true}.paint(Truncate(o.Title, cw, "…")), w, bg),
	}
	if o.Body != "" {
		lines = append(lines, row(bar+"    "+fg(t.Text.Secondary).paint(Truncate(o.Body, cw, "…")), w, bg))
	}
	if len(o.Hints) > 0 {
		lines = append(lines, row(bar+"    "+KeyHints(t, o.Hints, cw), w, bg))
	}
	lines = append(lines, row(bar, w, bg))
	return strings.Join(lines, "\n")
}

func toastIcon(t *theme.Theme, tone Tone) string {
	switch tone {
	case ToneSuccess:
		return t.Glyphs.Check
	case ToneDanger:
		return t.Glyphs.Cross
	case ToneWarning:
		return t.Glyphs.Blocked
	default:
		return t.Glyphs.ChevronRight
	}
}
