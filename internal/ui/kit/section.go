package kit

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// SectionOptions describes a Section header.
type SectionOptions struct {
	// Width is the exact width in cells.
	Width int
	// Label is the section name; it is shown in upper case.
	Label string
	// Meta is optional right-aligned plain text (a count, "3/7").
	Meta string
	// MetaTone colours Meta; the default is faint.
	MetaTone Tone
}

// Section renders a section header line: an upper-case muted bold label on
// the left, metadata on the right, one cell of padding each side
// (" SESSIONS            3 ").
func Section(t *theme.Theme, o SectionOptions) string {
	if o.Width <= 0 {
		return ""
	}
	label := ink{fg: t.Text.Muted, bold: true}.paint(strings.ToUpper(o.Label))
	meta := ""
	if o.Meta != "" {
		tone := o.MetaTone
		if tone == ToneDefault {
			tone = ToneFaint
		}
		meta = fg(ToneColor(t, tone)).paint(o.Meta)
	}
	if o.Width < 3 {
		return Fit("", o.Width)
	}
	return " " + spread(label, meta, o.Width-2) + " "
}
