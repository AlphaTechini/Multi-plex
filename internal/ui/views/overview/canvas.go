package overview

import (
	"image/color"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// pad is the horizontal inset of Overview content (docs/16: text content
// starts 2 cells in from the frame).
const pad = 2

// canvas collects the finished rows of the view. Every row is exactly w cells
// and painted, so nothing downstream has to fit or backfill it.
type canvas struct {
	th    *theme.Theme
	w     int
	base  color.Color
	rows  []string
	hits  []hit
	blank string
}

func newCanvas(th *theme.Theme, w int, rows []string, hits []hit) *canvas {
	base := th.Surface.Base
	c := &canvas{th: th, w: w, base: base, rows: rows[:0], hits: hits[:0]}
	c.blank = kit.Backfill(strings.Repeat(" ", w), base)
	return c
}

// y is the index the next row will get.
func (c *canvas) y() int { return len(c.rows) }

// gap adds an empty row.
func (c *canvas) gap() { c.rows = append(c.rows, c.blank) }

// text adds a row of content inset by pad, fitted and backfilled.
func (c *canvas) text(s string) {
	c.rows = append(c.rows, kit.Backfill(kit.Fit(strings.Repeat(" ", pad)+s, c.w), c.base))
}

// block adds an already painted, exactly (w-2*pad)-wide row (a card row) with
// the base surface on both sides.
func (c *canvas) block(s string) {
	edge := kit.Backfill(strings.Repeat(" ", pad), c.base)
	c.rows = append(c.rows, edge+s+edge)
}

// hit records a clickable region on the row about to be added.
func (c *canvas) hit(h hit) {
	h.y = c.y()
	c.hits = append(c.hits, h)
}

// section adds a section header ("SUMMARY ... meta") aligned to the content
// column.
func (c *canvas) section(label, meta string, tone kit.Tone) {
	s := kit.Section(c.th, kit.SectionOptions{Width: c.w - 2*pad + 2, Label: label, Meta: meta, MetaTone: tone})
	c.text(strings.TrimPrefix(s, " "))
}

// done returns exactly h rows, padding with blank rows and clipping extra.
func (c *canvas) done(h int) string {
	for len(c.rows) < h {
		c.rows = append(c.rows, c.blank)
	}
	return strings.Join(c.rows[:h], "\n")
}
