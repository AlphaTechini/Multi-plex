package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// View renders the whole screen.
func (m *Model) View() tea.View {
	content := ""
	if m.w > 0 && m.h > 0 {
		content = m.render()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.ReportFocus = true
	v.WindowTitle = m.windowTitle()
	if m.bench.on && m.w > 0 && !m.bench.once {
		m.bench.once = true
		close(m.bench.drawn)
	}
	return v
}

// windowTitle is "multiplex · <workspace>".
func (m *Model) windowTitle() string {
	name := m.ws.Name
	if name == "" {
		name = baseName(m.o.WorkspaceDir)
	}
	if name == "" {
		return "multiplex"
	}
	return "multiplex · " + name
}

func baseName(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// render builds the screen as an ANSI string of exactly w×h cells.
func (m *Model) render() string {
	if m.lay.tooSmall {
		return kit.EmptyState(m.th, kit.EmptyStateOptions{
			Width: m.w, Height: m.h,
			Title: "Terminal too small",
			Body:  fmt.Sprintf("Multiplex needs at least %d×%d. This one is %d×%d.", minWidth, minHeight, m.w, m.h),
		})
	}
	l := m.lay
	blocks := make([][]string, 0, 3)
	for _, r := range l.order {
		switch r {
		case regionSidebar:
			blocks = append(blocks, strings.Split(m.cachedSidebar(), "\n"))
		case regionMain:
			blocks = append(blocks, strings.Split(m.cachedMain(), "\n"))
		case regionAgent:
			blocks = append(blocks, strings.Split(m.cachedAgent(l.agentW, l.agentRail, &m.agentCache), "\n"))
		}
	}
	screen := joinExact(blocks...) + "\n" + m.renderStatus()
	if l.drawer {
		screen = kit.Composite(screen, m.cachedAgent(l.drawerW, false, &m.drawerCache), l.w-l.drawerW, 0)
	}
	screen = m.compositeToasts(screen)
	for _, o := range m.overlays {
		screen = o.view(m, screen)
	}
	return screen
}
