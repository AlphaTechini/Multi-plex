package pane

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// renderCache remembers the last rendered block so repeated frames with no
// new data cost nothing.
type renderCache struct {
	out     string
	version uint64
	w, h    int
	focused bool
	valid   bool
}

// Render draws the pane at exactly w×h cells on the panel surface: rows are
// clipped or padded to the size, foreign colours are backfilled onto
// Surface.Base, and when focused the cursor is a reverse-video cell. An
// exited pane keeps its last screen and shows a notice on the bottom row.
func (p *Pane) Render(w, h int, focused bool) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	c := &p.cache
	if c.valid && c.version == p.version && c.w == w && c.h == h && c.focused == focused {
		return c.out
	}
	lines := make([]string, h)
	for y := range h {
		row := ""
		if y < len(p.rows) {
			row = p.rows[y]
		}
		lines[y] = kit.PadRight(kit.Truncate(row, w, ""), w)
	}
	if focused && p.cursorOn && !p.exited && p.cy >= 0 && p.cy < h && p.cx >= 0 && p.cx < w {
		lines[p.cy] = withCursor(lines[p.cy], p.cx, w)
	}
	if p.exited {
		lines[h-1] = p.exitedRow(w)
	}
	out := kit.Backfill(strings.Join(lines, "\n"), p.th.Surface.Base)
	*c = renderCache{out: out, version: p.version, w: w, h: h, focused: focused, valid: true}
	return out
}

// withCursor draws a reverse-video cell at column x of a w-cell line.
func withCursor(line string, x, w int) string {
	left := ansi.Cut(line, 0, x)
	cell := ansi.Cut(line, x, x+1)
	if ansi.StringWidth(cell) == 0 {
		cell = " "
	}
	right := ansi.Cut(line, x+1, w)
	return left + "\x1b[0m\x1b[7m" + ansi.Strip(cell) + "\x1b[0m" + right
}

// exitedRow is the notice shown over the bottom row of a finished pane.
func (p *Pane) exitedRow(w int) string {
	t := p.th
	msg := p.name + " exited"
	if p.exitCode != 0 {
		msg += " (code " + itoa(p.exitCode) + ")"
	}
	line := " " + paint(t.State.Error, t.Glyphs.Cross) + " " + paint(t.Text.Secondary, msg) +
		paint(t.Text.Faint, " "+t.Glyphs.Dot+" ") + paint(t.Text.Strong, "r") + " " + paint(t.Text.Muted, "restart")
	return kit.Backfill(kit.Fit(line, w), t.Surface.Sunken)
}
