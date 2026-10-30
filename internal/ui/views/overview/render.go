package overview

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// Render draws the view at exactly width×height cells. It is a pure
// function of the model except that it records the clickable rectangles, the
// card window and the file-link count the key and mouse handlers use.
func (m *model) Render(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	th := m.th
	if m.sess == nil {
		return kit.EmptyState(th, kit.EmptyStateOptions{
			Width: width, Height: height,
			Eyebrow: "overview",
			Title:   "No session selected",
			Body:    "Pick a session in the sidebar to see its report, insight cards and warnings here.",
			Hints:   []kit.Hint{{Keys: "n", Label: "new session"}},
		})
	}
	c := newCanvas(th, width, m.rows, m.hits)
	cw := width - 2*pad
	if cw < 12 {
		m.header(c, max(cw, 1))
		m.nFiles, m.nVisFiles = 0, 0
		out := c.done(height)
		m.rows, m.hits = c.rows, c.hits
		return out
	}
	m.header(c, cw)
	m.summary(c, cw, height)
	m.insights(c, cw, height)
	out := c.done(height)
	m.rows, m.hits = c.rows, c.hits
	return out
}

var _ view.View = (*model)(nil)

// sectionLine renders a section header at content width, without the inset.
func (m *model) sectionLine(cw int, label, meta string, tone kit.Tone) string {
	s := kit.Section(m.th, kit.SectionOptions{Width: cw + 2, Label: label, Meta: meta, MetaTone: tone})
	return strings.TrimPrefix(s, " ")
}
