package codeview

import (
	tea "charm.land/bubbletea/v2"
)

// wheelStep is how many lines one wheel notch scrolls.
const wheelStep = 3

// onWheel scrolls the viewport; it pauses following like any scroll.
func (m *model) onWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if m.picker != nil {
		switch msg.Mouse().Button {
		case tea.MouseWheelUp:
			m.picker.move(-1)
		case tea.MouseWheelDown:
			m.picker.move(1)
		}
		return nil
	}
	if m.src == nil {
		return nil
	}
	switch msg.Mouse().Button {
	case tea.MouseWheelUp:
		m.scrollBy(-wheelStep)
	case tea.MouseWheelDown:
		m.scrollBy(wheelStep)
	default:
		return nil
	}
	m.pauseFollow()
	return m.request()
}

// onClick moves the cursor to the clicked line, or opens the go-to-file box
// from the banner. Clicks never reach the agent.
func (m *model) onClick(msg tea.MouseClickMsg) tea.Cmd {
	mm := msg.Mouse()
	if m.sess == nil || mm.Button != tea.MouseLeft || m.picker != nil {
		return nil
	}
	switch {
	case mm.Y == 0 && m.path != "":
		m.picker = newPicker(m.th, m.changes)
	case mm.Y >= headerRows && m.src != nil:
		line := m.top + mm.Y - headerRows
		if line < m.lines() {
			m.cursor = line
			m.pauseFollow()
		}
	}
	return nil
}
