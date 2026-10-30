package app

import (
	tea "charm.land/bubbletea/v2"
)

// overlay is a modal layer over the screen: the command palette, the
// new-session dialog, the approval prompt or the help. The topmost overlay
// owns the keyboard.
type overlay interface {
	// update handles a key press, paste or mouse event, or an overlay-specific
	// message. It returns nil to close the overlay.
	update(m *Model, msg tea.Msg) (overlay, tea.Cmd)
	// view draws the overlay over the given screen.
	view(m *Model, screen string) string
}

// pushOverlay opens an overlay on top of the others.
func (m *Model) pushOverlay(o overlay) {
	m.prefixOn = false
	m.overlays = append(m.overlays, o)
}

// popOverlay closes the topmost overlay.
func (m *Model) popOverlay() {
	if n := len(m.overlays); n > 0 {
		m.overlays = m.overlays[:n-1]
	}
}

// overlayInput sends a key, paste or mouse message to the topmost overlay.
func (m *Model) overlayInput(msg tea.Msg) tea.Cmd {
	n := len(m.overlays)
	if n == 0 {
		return nil
	}
	top := m.overlays[n-1]
	next, cmd := top.update(m, msg)
	// The overlay may have closed itself and pushed another one before
	// returning, so find it by identity.
	for i, o := range m.overlays {
		if o != top {
			continue
		}
		if next == nil {
			m.overlays = append(m.overlays[:i], m.overlays[i+1:]...)
		} else {
			m.overlays[i] = next
		}
		break
	}
	return cmd
}

// overlayMouse lets the topmost overlay see mouse events.
func (m *Model) overlayMouse(msg tea.MouseMsg) tea.Cmd {
	return m.overlayInput(msg)
}

// routeToOverlay delivers an asynchronous result to the topmost overlay.
func (m *Model) routeToOverlay(msg tea.Msg) tea.Cmd { return m.overlayInput(msg) }
