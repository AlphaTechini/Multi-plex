package app

import (
	tea "charm.land/bubbletea/v2"
)

// layoutInput gathers the layout's inputs from the model.
func (m *Model) layoutInput() layoutInput {
	return layoutInput{
		w: m.w, h: m.h, mode: m.mode, zoom: m.zoom, zoomRegion: m.focus,
		agentHidden: m.agentHides, agentDrawer: m.agentDraw, agentLeft: m.agentLeft,
		panelShare: m.o.PanelWidth, panelMinCols: m.o.PanelMinCols,
	}
}

// recompute recalculates the layout, and moves focus off a region that is not
// shown. The drawer closes when focus leaves the agent panel.
func (m *Model) recompute() {
	m.lay = computeLayout(m.layoutInput())
	if m.lay.tooSmall {
		return
	}
	if m.focus == regionAgent && !m.lay.shown(regionAgent) {
		m.focus = regionMain
		m.agentDraw = false
		m.lay = computeLayout(m.layoutInput())
	}
	if m.agentDraw && m.focus != regionAgent {
		m.agentDraw = false
		m.lay = computeLayout(m.layoutInput())
	}
}

// announce tells the active view and the agent pane about size changes.
func (m *Model) announce() tea.Cmd {
	if m.lay.tooSmall {
		return nil
	}
	var cmds []tea.Cmd
	if cols, rows := m.lay.mainInner(); [2]int{cols, rows} != m.lastMain && cols > 0 && rows > 0 {
		m.lastMain = [2]int{cols, rows}
		// Every built view learns the new size; others re-learn on activation.
		for mode := range m.views {
			cmds = append(cmds, m.sendView(mode, tea.WindowSizeMsg{Width: cols, Height: rows}))
		}
	}
	if cols, rows := m.lay.agentInner(); [2]int{cols, rows} != m.lastAgent && cols > 0 && rows > 0 && (m.lay.drawer || !m.lay.agentRail) {
		m.lastAgent = [2]int{cols, rows}
		if p := m.agentPaneOf(m.session()); p != nil {
			cmds = append(cmds, p.Resize(cols, rows))
		}
	}
	return tea.Batch(cmds...)
}

// setFocus moves keyboard focus to a region.
func (m *Model) setFocus(r region) tea.Cmd {
	if r == m.focus {
		return nil
	}
	m.focus = r
	if r != regionAgent {
		m.agentDraw = false
	}
	m.recompute()
	return tea.Batch(m.syncViewFocus(), m.announce())
}

// moveFocus moves focus to the neighbouring visible region (delta -1 left,
// +1 right). Moving onto a collapsed agent rail opens the drawer.
func (m *Model) moveFocus(delta int) tea.Cmd {
	order := m.lay.order
	i := 0
	for j, r := range order {
		if r == m.focus {
			i = j
		}
	}
	j := i + delta
	if j < 0 || j >= len(order) {
		return nil
	}
	if order[j] == regionAgent && !m.lay.shown(regionAgent) {
		return m.toggleAgent()
	}
	return m.setFocus(order[j])
}

// toggleAgent opens or collapses the agent panel: it collapses the docked
// panel to a rail, and opens the drawer when it is already a rail.
func (m *Model) toggleAgent() tea.Cmd {
	l := m.lay
	switch {
	case l.drawer:
		m.agentDraw = false
		return m.setFocus(regionMain)
	case l.agentRail:
		natural := computeLayout(layoutInput{w: m.w, h: m.h, mode: m.mode, panelShare: m.o.PanelWidth, panelMinCols: m.o.PanelMinCols})
		if !natural.agentRail && m.agentHides {
			m.agentHides = false // re-dock the collapsed panel
			m.recompute()
			return tea.Batch(m.setFocus(regionAgent), m.announce())
		}
		m.agentDraw = true
		m.focus = regionAgent
		m.recompute()
		return tea.Batch(m.syncViewFocus(), m.announce())
	default:
		m.agentHides = true
		if m.focus == regionAgent {
			m.focus = regionMain
		}
		m.recompute()
		return tea.Batch(m.syncViewFocus(), m.announce())
	}
}

// toggleZoom makes the focused region fill the screen, or restores the layout.
func (m *Model) toggleZoom() tea.Cmd {
	m.zoom = !m.zoom
	m.recompute()
	return m.announce()
}

// swapAgentSide moves the agent panel to the other side of the main view.
func (m *Model) swapAgentSide() tea.Cmd {
	m.agentLeft = !m.agentLeft
	m.recompute()
	return m.announce()
}

// cycleMode moves to the next (delta 1) or previous (-1) mode.
func (m *Model) cycleMode(delta int) tea.Cmd {
	i := 0
	for j, mode := range modeOrder {
		if mode == m.mode {
			i = j
		}
	}
	return m.setMode(modeOrder[(i+delta+len(modeOrder))%len(modeOrder)])
}
