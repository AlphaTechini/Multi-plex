package kit

import (
	"fmt"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// ProgressOptions describes a Progress bar.
type ProgressOptions struct {
	// Width is the exact width in cells, label included.
	Width int
	// Done and Total give the fraction.
	Done, Total int
	// Tone colours the filled part; the default is ToneAccent.
	Tone Tone
	// Label appends " 3/7" after the bar.
	Label bool
}

// Progress renders a thin progress line: the filled part in the tone colour,
// the track in the border colour ("━━━━━━────── 3/7").
func Progress(t *theme.Theme, o ProgressOptions) string {
	w := o.Width
	if w <= 0 {
		return ""
	}
	label := ""
	if o.Label {
		label = fmt.Sprintf(" %d/%d", o.Done, o.Total)
	}
	bw := w - len(label)
	if bw < 1 {
		return Fit(label, w)
	}
	fill := 0
	if o.Total > 0 {
		fill = min(max(o.Done*bw/o.Total, 0), bw)
	}
	tone := o.Tone
	if tone == ToneDefault {
		tone = ToneAccent
	}
	return fg(ToneColor(t, tone)).paint(repeat(t.Glyphs.ProgressFill, fill)) +
		fg(t.Border.Default).paint(repeat(t.Glyphs.ProgressTrack, bw-fill)) +
		fg(t.Text.Muted).paint(label)
}
