package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// handleMouse routes a mouse event: toasts, the status bar's approvals pill,
// then the region under the pointer. Coordinates passed to the main view are
// relative to its top-left corner inside the frame.
func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if m.lay.tooSmall || m.w == 0 {
		return nil
	}
	mm := msg.Mouse()
	if len(m.overlays) > 0 {
		return m.overlayMouse(msg)
	}
	_, isClick := msg.(tea.MouseClickMsg)
	if isClick && mm.Button == tea.MouseLeft {
		if id := m.toastAt(mm.X, mm.Y); id != 0 {
			m.removeToast(id)
			return nil
		}
		if mm.Y == m.h-1 {
			return m.statusClick(mm.X)
		}
	}
	r, ok := m.lay.regionAt(mm.X)
	if !ok || mm.Y >= m.lay.bodyH {
		return nil
	}
	switch msg.(type) {
	case tea.MouseWheelMsg:
		return m.wheel(r, msg)
	case tea.MouseClickMsg:
		if mm.Button != tea.MouseLeft {
			return nil
		}
		focus := m.setFocus(r)
		return tea.Batch(focus, m.regionClick(r, mm))
	case tea.MouseReleaseMsg, tea.MouseMotionMsg:
		if r == regionMain && m.focus == regionMain && m.loaded && !m.welcome() && mm.Y >= 1 {
			return m.sendView(m.mode, relativeMouse(msg, m.lay.x(regionMain)+1, 1))
		}
	}
	return nil
}

// wheel scrolls the sidebar, or forwards the wheel to the main view.
func (m *Model) wheel(r region, msg tea.MouseMsg) tea.Cmd {
	mm := msg.Mouse()
	switch r {
	case regionSidebar:
		d := 3
		if mm.Button == tea.MouseWheelUp {
			d = -3
		}
		iw, ih := max(m.lay.sidebarW-2, 0), max(m.lay.bodyH-2, 0)
		m.sb.scroll = min(max(m.sb.scroll+d, 0), max(len(m.sbLines(iw))-ih, 0))
	case regionMain:
		if m.loaded && !m.welcome() && mm.Y >= 1 {
			return m.sendView(m.mode, relativeMouse(msg, m.lay.x(regionMain)+1, 1))
		}
	}
	return nil
}

// relativeMouse re-expresses a mouse message with the origin moved to (ox, oy).
func relativeMouse(msg tea.MouseMsg, ox, oy int) tea.Msg {
	mm := msg.Mouse()
	mm.X -= ox
	mm.Y -= oy
	switch msg.(type) {
	case tea.MouseClickMsg:
		return tea.MouseClickMsg(mm)
	case tea.MouseReleaseMsg:
		return tea.MouseReleaseMsg(mm)
	case tea.MouseWheelMsg:
		return tea.MouseWheelMsg(mm)
	}
	return tea.MouseMotionMsg(mm)
}

// regionClick acts on a left click inside a region.
func (m *Model) regionClick(r region, mm tea.Mouse) tea.Cmd {
	switch r {
	case regionSidebar:
		iw := max(m.lay.sidebarW-2, 0)
		lines := m.sbLines(iw)
		i := mm.Y - 1 + m.sb.scroll
		if mm.Y >= 1 && i >= 0 && i < len(lines) && lines[i].item >= 0 {
			return m.sbActivate(lines[i].item)
		}
	case regionMain:
		x0 := m.lay.x(regionMain)
		if mm.Y == 0 {
			if mode, ok := m.tabAt(mm.X - x0); ok {
				return m.actMode(mode)
			}
			return nil
		}
		if m.loaded && !m.welcome() {
			return m.sendView(m.mode, relativeMouse(tea.MouseClickMsg(mm), x0+1, 1))
		}
	}
	return nil
}

// tabAt maps a column inside the main panel's top edge to a mode. Tabs start
// three cells in ("╭─ "), one cell of padding each side, one cell between.
func (m *Model) tabAt(x int) (view.Mode, bool) {
	if m.welcome() {
		return 0, false
	}
	pos := 3
	for i, t := range m.tabs() {
		w := 1 + ansi.StringWidth(t.Label) + 1
		if t.Badge != "" {
			w += 1 + ansi.StringWidth(t.Badge)
		}
		if x >= pos && x < pos+w {
			return modeOrder[i], true
		}
		pos += w + 1
	}
	return 0, false
}

// statusClick opens the approval prompt when the approvals pill is clicked.
func (m *Model) statusClick(x int) tea.Cmd {
	pill := m.approvalPill()
	if pill == "" {
		return nil
	}
	// The pill starts with the (possibly blinked-off) dot cell; find the rest.
	suffix := pill[strings.Index(pill, " "):]
	plain := ansi.Strip(m.renderStatus())
	i := strings.Index(plain, suffix)
	if i < 0 {
		return nil
	}
	start := ansi.StringWidth(plain[:i]) - 1
	if x >= start-1 && x <= start+ansi.StringWidth(pill)+1 {
		return m.actApprovals()
	}
	return nil
}
