package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// handleKey routes a key press. An overlay owns the keyboard while it is
// open. Otherwise the prefix key starts a Multiplex command; every other key
// goes to the focused region, and the agent panel and capturing views get all
// of them.
func (m *Model) handleKey(k tea.KeyPressMsg) tea.Cmd {
	if len(m.overlays) > 0 {
		return m.overlayInput(k)
	}
	key := k.String()
	if m.prefixOn {
		m.prefixOn = false
		return m.prefixCommand(k)
	}
	if key == m.prefix {
		m.prefixOn = true
		return nil
	}
	switch m.focus {
	case regionAgent:
		return m.agentKey(k)
	case regionMain:
		return m.mainKey(k)
	}
	return m.sidebarKey(k)
}

// handlePaste sends pasted text to the focused terminal-like region.
func (m *Model) handlePaste(p tea.PasteMsg) tea.Cmd {
	if len(m.overlays) > 0 {
		return m.overlayInput(p)
	}
	switch m.focus {
	case regionAgent:
		if ap := m.agentPaneOf(m.session()); ap != nil {
			ap.Paste(p.Content)
		}
	case regionMain:
		return m.sendView(m.mode, p)
	}
	return nil
}

// forwardKeyToFocus sends k to whatever has focus without interpreting it
// (used for the literal prefix key).
func (m *Model) forwardKeyToFocus(k tea.KeyPressMsg) tea.Cmd {
	switch m.focus {
	case regionAgent:
		if ap := m.agentPaneOf(m.session()); ap != nil {
			ap.Key(k)
		}
	case regionMain:
		if m.loaded && m.active().Capturing() {
			return m.sendView(m.mode, k)
		}
	}
	return nil
}

// prefixCommand runs the key pressed after the prefix.
func (m *Model) prefixCommand(k tea.KeyPressMsg) tea.Cmd {
	key := k.String()
	switch key {
	case "esc":
		return nil
	case m.prefix:
		return m.forwardKeyToFocus(k) // prefix twice sends the key itself
	}
	if a, ok := m.prefixAction(key); ok {
		return a.run(m)
	}
	return nil
}

// agentKey sends keys to the agent's terminal. When the agent has exited, "r"
// restarts the session.
func (m *Model) agentKey(k tea.KeyPressMsg) tea.Cmd {
	s := m.session()
	p := m.agentPaneOf(s)
	if p == nil {
		return nil
	}
	if p.Key(k) {
		return nil
	}
	if exited, _ := p.Exited(); exited && k.String() == "r" {
		return m.actRestart()
	}
	return nil
}

// mainKey handles keys while the main view is focused. A capturing view gets
// everything; otherwise the shell keeps tab, ?, ctrl+c and the welcome-screen
// shortcuts and the view gets the rest.
func (m *Model) mainKey(k tea.KeyPressMsg) tea.Cmd {
	if !m.loaded {
		return nil
	}
	if m.welcome() {
		switch k.String() {
		case "n":
			return m.actNewSession()
		case "?":
			return m.actHelp()
		case "q", "ctrl+c":
			return m.actQuit()
		case "tab":
			return m.cycleMode(1)
		case "h", "left":
			return m.setFocus(regionSidebar)
		}
		return nil
	}
	v := m.active()
	if !v.Capturing() {
		switch k.String() {
		case "tab":
			return m.cycleMode(1)
		case "shift+tab":
			return m.cycleMode(-1)
		case "?":
			return m.actHelp()
		case "ctrl+c":
			return m.actQuit()
		}
	}
	return m.sendView(m.mode, k)
}

// sidebarKey handles the sidebar's single-key shortcuts.
func (m *Model) sidebarKey(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "j", "down":
		return m.sbMove(1)
	case "k", "up":
		return m.sbMove(-1)
	case "g", "home":
		return m.sbMoveTo(0)
	case "G", "end":
		return m.sbMoveTo(len(m.sb.items) - 1)
	case "enter", "space":
		return m.sbActivate(m.sb.cursor)
	case "l", "right", "esc":
		return m.setFocus(regionMain)
	case "n":
		return m.actNewSession()
	case "?":
		return m.actHelp()
	case "/":
		return m.actPalette()
	case "tab":
		return m.cycleMode(1)
	case "shift+tab":
		return m.cycleMode(-1)
	case "q", "ctrl+c":
		return m.actQuit()
	}
	return nil
}

// sbMove moves the sidebar cursor; landing on a session selects it.
func (m *Model) sbMove(delta int) tea.Cmd {
	return m.sbMoveTo(m.sb.cursor + delta)
}

func (m *Model) sbMoveTo(i int) tea.Cmd {
	if len(m.sb.items) == 0 {
		return nil
	}
	m.sb.cursor = min(max(i, 0), len(m.sb.items)-1)
	m.sbEnsureVisible()
	if it := m.sb.items[m.sb.cursor]; it.kind == sbSession && it.id != m.cur {
		return m.selectSession(it.id)
	}
	return nil
}

// sbActivate opens a sidebar entry: a session takes the main view's focus, a
// spec opens in Specs, a steering file opens read-only in Code.
func (m *Model) sbActivate(i int) tea.Cmd {
	if i < 0 || i >= len(m.sb.items) {
		return nil
	}
	m.sb.cursor = i
	it := m.sb.items[i]
	switch it.kind {
	case sbSession:
		return tea.Batch(m.selectSession(it.id), m.setFocus(regionMain))
	case sbSpec:
		return m.switchMode(view.SwitchModeMsg{Mode: view.ModeSpecs, Path: it.spec})
	default:
		return m.openSteering(it.steer)
	}
}
