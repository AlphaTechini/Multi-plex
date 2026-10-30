package specview

import (
	tea "charm.land/bubbletea/v2"
)

// onClick selects the clicked spec in the list or the clicked document tab.
func (m *model) onClick(msg tea.MouseClickMsg) tea.Cmd {
	mm := msg.Mouse()
	if mm.Button != tea.MouseLeft || len(m.specs) == 0 {
		return nil
	}
	if m.twoPane && mm.X < m.listW || !m.twoPane && !m.detail {
		const rowsPer = 3
		if i := (mm.Y - m.listY) / rowsPer; mm.Y >= m.listY && i >= 0 && i < len(m.specs) {
			if !m.twoPane {
				m.detail = true
			}
			return m.selectSpec(i)
		}
		return nil
	}
	if mm.Y == tabRow {
		for k, x := range m.tabX {
			if mm.X >= x[0] && mm.X < x[1] {
				m.setKind(k)
			}
		}
	}
	return nil
}

// onWheel scrolls the document, or moves the list selection over the list.
func (m *model) onWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if len(m.specs) == 0 {
		return nil
	}
	mm := msg.Mouse()
	d := 0
	switch mm.Button {
	case tea.MouseWheelUp:
		d = -1
	case tea.MouseWheelDown:
		d = 1
	default:
		return nil
	}
	if m.twoPane && mm.X < m.listW || !m.twoPane && !m.detail {
		return m.selectSpec(m.sel + d)
	}
	m.scrollBy(3 * d)
	return nil
}
