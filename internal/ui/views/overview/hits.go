package overview

import "github.com/alphatechini/multi-plex/internal/core"

// hitKind says what a clickable region does.
type hitKind uint8

const (
	hitCard      hitKind = iota // select a card
	hitRef                      // open a card reference in the Code view
	hitAction                   // run a card action
	hitFile                     // open a summary file in the Diff view
	hitMoreFiles                // "+N more": open the Diff view
	hitEarlier                  // "↑ N earlier": move the selection up
	hitLater                    // "↓ N more": move the selection down
)

// hit is one clickable rectangle (one row tall) recorded by Render and used
// by the mouse handler. Coordinates are relative to the view.
type hit struct {
	y, x0, x1 int
	kind      hitKind
	card      int // index into the visible card list
	file      int // index into the visible file links
	path      string
	line      int
	action    core.NoteAction
}

// at returns the first hit covering (x, y). Specific regions are recorded
// before the card-wide ones, so they win.
func hitAt(hits []hit, x, y int) (hit, bool) {
	for _, h := range hits {
		if h.y == y && x >= h.x0 && x < h.x1 {
			return h, true
		}
	}
	return hit{}, false
}
