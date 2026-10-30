package codeview

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// headerRows is the banner and the status row above the code.
const headerRows = 2

var _ view.View = (*model)(nil)

// Render draws the view at exactly width×height cells. Only the visible
// lines are touched, and highlighting comes from the Source's cache: a line
// whose chunk has not arrived is drawn plain. Render records the viewport
// height for scrolling, so it is not strictly pure.
func (m *model) Render(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	th := m.th
	if m.sess == nil {
		return kit.EmptyState(th, kit.EmptyStateOptions{
			Width: width, Height: height, Eyebrow: "code",
			Title: "No session selected",
			Body:  "Pick a session in the sidebar to read its files and follow the agent's edits.",
		})
	}
	if height < headerRows+1 || width < 12 {
		return kit.Backfill(kit.FitBlock(clean(m.path, true), width, height), th.Surface.Base)
	}
	m.rows = height - headerRows
	m.clampScroll()

	var out string
	switch {
	case m.path == "" || (m.src == nil && m.note == ""):
		out = m.renderEmpty(width, height)
	case m.src == nil:
		out = m.renderNote(width, height)
	default:
		out = m.renderCode(width, height)
	}
	if m.picker != nil {
		out = m.renderPicker(out, width, height)
	}
	return out
}

// renderCode draws banner, status row and the visible lines.
func (m *model) renderCode(w, h int) string {
	th := m.th
	var b strings.Builder
	b.Grow(w * h * 4)
	b.WriteString(m.banner(w))
	b.WriteByte('\n')
	b.WriteString(m.statusRow(w))

	n := m.src.NumLines()
	nw := max(len(itoa(n)), 3)
	cw := w - (nw + 4)
	hl := m.freshLevel()
	blank := kit.Backfill(strings.Repeat(" ", w), th.Surface.Base)
	for r := 0; r < h-headerRows; r++ {
		b.WriteByte('\n')
		i := m.top + r
		if i >= n || cw < 1 {
			b.WriteString(blank)
			continue
		}
		m.codeLine(&b, i, nw, cw, hl)
	}
	return b.String()
}
