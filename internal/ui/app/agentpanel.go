package app

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// renderAgent draws the agent panel (docked, as a rail, or as the drawer's
// content) at width w.
func (m *Model) renderAgent(w int, rail bool) string {
	l := m.lay
	t := m.th
	s := m.session()
	if rail {
		var lines []string
		if s != nil {
			lines = []string{kit.StateDot(t, kitState(s.State), m.frame)}
		}
		return kit.Panel(t, kit.PanelOptions{Width: w, Height: l.bodyH, Flush: true, Content: strings.Join(lines, "\n")})
	}
	focused := m.focus == regionAgent
	po := kit.PanelOptions{Width: w, Height: l.bodyH, Focused: focused, Flush: true}
	iw, ih := kit.PanelInner(po)
	if s == nil {
		po.Title = "Agent"
		po.Content = kit.EmptyState(t, kit.EmptyStateOptions{
			Width: iw, Height: ih,
			Body: "The agent's own terminal appears here when a session starts.",
		})
		return kit.Panel(t, po)
	}
	po.Title = s.Slug
	po.Icon = kit.StateDot(t, kitState(s.State), m.frame)
	po.Meta = agentLabel(s.Agent)
	p := m.agentPaneOf(s)
	exited := false
	if p != nil {
		exited, _ = p.Exited()
	}
	hints := []kit.Hint{{Keys: "esc", Label: "interrupt"}, {Keys: m.prefix + " z", Label: "zoom"}}
	if exited {
		hints = []kit.Hint{{Keys: "r", Label: "restart"}, {Keys: m.prefix + " z", Label: "zoom"}}
	}
	po.Footer = kit.KeyHints(t, hints, 0)
	if p == nil {
		label := "Starting " + agentLabel(s.Agent) + "…"
		if s.AgentPane == "" && s.State != "starting" {
			label = "No agent terminal for this session"
		}
		po.Content = kit.EmptyState(t, kit.EmptyStateOptions{Width: iw, Height: ih, Body: label})
	} else {
		po.Content = p.Render(iw, ih, focused)
	}
	return kit.Panel(t, po)
}
