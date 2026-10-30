package kit

import (
	"fmt"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// FileEndOptions describes a FileEnd marker.
type FileEndOptions struct {
	// Width is the exact width in cells.
	Width int
	// Next is the next file's path; empty when this is the last file.
	Next string
	// Keys is the next-file key spec (shown as a key cap).
	Keys string
	// Total is the number of files, used in the last-file message.
	Total int
}

// FileEnd renders the end-of-file marker of the diff view. The view never
// scrolls into the next file; this row says where the file ends and how to
// move on: "──── end of file · next  src/auth/cache.go  ] ────".
func FileEnd(t *theme.Theme, o FileEndOptions) string {
	w := o.Width
	if w <= 0 {
		return ""
	}
	rule := fg(t.Border.Default)
	muted := fg(t.Text.Muted)
	var mid string
	if o.Next != "" {
		mid = muted.paint(" end of file  "+t.Glyphs.Dot+"  next ") + " " +
			fg(t.Text.Primary).paint(o.Next) + "  " + Keycap(t, o.Keys) + " "
	} else {
		mid = muted.paint(fmt.Sprintf(" end of file  %s  last of %d files ", t.Glyphs.Dot, o.Total))
	}
	mw := Width(mid)
	if mw+4 > w {
		return Fit(mid, w)
	}
	left := (w - mw) / 2
	return rule.paint(repeat(t.Glyphs.Rule, left)) + mid + rule.paint(repeat(t.Glyphs.Rule, w-left-mw))
}
