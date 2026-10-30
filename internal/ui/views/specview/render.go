package specview

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

var _ view.View = (*model)(nil)

const (
	// twoPaneMin is the width from which the list and the document sit side by
	// side; narrower, one at a time.
	twoPaneMin = 84
	// listWidth is the spec list's width in two-pane layout.
	listWidth = 30
	// pad is the horizontal inset of the document pane.
	pad = 2
	// tabRow is the row of the document tabs in the document pane.
	tabRow = 4
	// bodyTop is the first row of document text in the document pane.
	bodyTop = 7
)

// Render draws the view at exactly width×height cells. Markdown is rendered
// once per document and width and cached; a frame only slices it.
func (m *model) Render(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	th := m.th
	m.bodyH = max(height-bodyTop, 1)
	switch {
	case len(m.specs) == 0 && (m.list == notLoaded || m.list == loading):
		return kit.EmptyState(th, kit.EmptyStateOptions{Width: width, Height: height, Eyebrow: "specs", Title: "Loading specs…"})
	case m.list == failed && len(m.specs) == 0:
		return kit.EmptyState(th, kit.EmptyStateOptions{
			Width: width, Height: height, Eyebrow: "specs", Title: "Could not load the specs",
			Body: m.listErr, Hints: []kit.Hint{{Keys: "r", Label: "retry"}},
		})
	case len(m.specs) == 0:
		return kit.EmptyState(th, kit.EmptyStateOptions{
			Width: width, Height: height, Eyebrow: "specs",
			Title: "No specs yet",
			Body:  "Specs live in .multiplex/specs/<slug>/ as requirements.md, design.md and tasks.md. Start one with a planner session: describe the idea and it drafts the requirements for you to review.",
			Hints: []kit.Hint{{Keys: "n", Label: "new planner session"}},
		})
	}
	if width < 20 || height < 8 {
		return kit.Backfill(kit.FitBlock("Specs", width, height), th.Surface.Base)
	}
	m.twoPane = width >= twoPaneMin
	if m.twoPane {
		m.listW = listWidth
		sep := paint(th.Border.Subtle, false, th.Glyphs.Frame.Vertical)
		sepCol := kit.Backfill(strings.TrimSuffix(strings.Repeat(sep+"\n", height), "\n"), th.Surface.Base)
		return kit.JoinColumns(m.renderList(listWidth, height), sepCol, m.renderDetail(width-listWidth-1, height, listWidth+1))
	}
	m.listW = 0
	if m.detail {
		return m.renderDetail(width, height, 0)
	}
	return m.renderList(width, height)
}

// fitLine fits a styled line to w cells on the page surface.
func (m *model) fitLine(s string, w int) string {
	return kit.Backfill(kit.Fit(s, w), m.th.Surface.Base)
}

// pageLines pads or clips rows to exactly h lines of w cells.
func (m *model) pageLines(rows []string, w, h int) string {
	for len(rows) < h {
		rows = append(rows, kit.Backfill(strings.Repeat(" ", w), m.th.Surface.Base))
	}
	return strings.Join(rows[:h], "\n")
}

func cells(s string) int { return ansi.StringWidth(s) }
