package app

import (
	"fmt"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// metaer is implemented by views that show text at the right end of the main
// panel's top edge (Overview: "updated 32s ago"; Diff: "unified · 7 files").
type metaer interface{ Meta() string }

// tabs returns the mode tabs. Views that are not built yet show their name.
func (m *Model) tabs() []kit.Tab {
	if m.welcome() {
		return []kit.Tab{{Label: "Welcome"}}
	}
	tabs := make([]kit.Tab, len(modeOrder))
	for i, mode := range modeOrder {
		tabs[i] = kit.Tab{Label: modeName(mode)}
		if v, ok := m.views[mode]; ok {
			label, badge := v.Tab()
			tabs[i].Label = label
			if badge > 0 {
				tabs[i].Badge = itoa(badge)
			}
		}
	}
	return tabs
}

// activeTab is the index of the highlighted tab.
func (m *Model) activeTab() int {
	if m.welcome() {
		return 0
	}
	for i, mode := range modeOrder {
		if mode == m.mode {
			return i
		}
	}
	return 0
}

// mainMeta is the text at the right of the main panel's top edge.
func (m *Model) mainMeta() string {
	if m.welcome() || !m.loaded {
		return ""
	}
	if v, ok := m.views[m.mode].(metaer); ok {
		return v.Meta()
	}
	if m.mode == view.ModeOverview {
		if s := m.session(); s != nil && !s.UpdatedAt.IsZero() {
			return fmt.Sprintf("updated %s ago", age(m.now().Sub(s.UpdatedAt)))
		}
	}
	return ""
}

// renderMain draws the main panel: the tab bar in the top edge and the active
// mode (or the welcome / loading screen) inside.
func (m *Model) renderMain() string {
	l := m.lay
	t := m.th
	po := kit.PanelOptions{
		Width: l.mainW, Height: l.bodyH,
		Header:  kit.TabBar(t, kit.TabBarOptions{Tabs: m.tabs(), Active: m.activeTab()}),
		Meta:    m.mainMeta(),
		Focused: m.focus == regionMain,
		Flush:   true,
	}
	iw, ih := kit.PanelInner(po)
	switch {
	case !m.loaded:
		body := "Connecting to the daemon…"
		if m.loadErr != "" {
			body = m.loadErr
		}
		po.Content = kit.EmptyState(t, kit.EmptyStateOptions{Width: iw, Height: ih, Title: "Multiplex", Body: body})
	case m.welcome():
		po.Content = kit.EmptyState(t, kit.EmptyStateOptions{
			Width: iw, Height: ih,
			Eyebrow: "m u l t i p l e x",
			Title:   "No sessions yet",
			Body:    "A session is one agent working on one task in its own worktree and branch. Start one and its summary, diff and terminal appear here.",
			Hints: []kit.Hint{
				{Keys: "n", Label: "new session"},
				{Keys: m.prefix + " p", Label: "command palette"},
				{Keys: "?", Label: "all keys"},
			},
		})
	default:
		po.Content = m.active().Render(iw, ih)
	}
	return kit.Panel(t, po)
}
