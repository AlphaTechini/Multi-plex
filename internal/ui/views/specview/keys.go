package specview

import (
	tea "charm.land/bubbletea/v2"
)

// onKey handles a key press.
func (m *model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	if len(m.specs) == 0 {
		if msg.String() == "r" {
			m.list = loading
			return specsCmd(m.be, m.ws.ID)
		}
		return nil
	}
	page := max(m.bodyH-1, 1)
	switch msg.String() {
	case "up":
		if !m.twoPane && m.detail {
			m.scrollBy(-1)
			return nil
		}
		return m.selectSpec(m.sel - 1)
	case "down":
		if !m.twoPane && m.detail {
			m.scrollBy(1)
			return nil
		}
		return m.selectSpec(m.sel + 1)
	case "enter":
		m.detail = true
	case "esc":
		m.detail = false
	case "left", "h", "shift+tab":
		m.setKind((int(m.kind) + 2) % 3)
	case "right", "l", "tab":
		m.setKind((int(m.kind) + 1) % 3)
	case "1":
		m.setKind(int(docRequirements))
	case "2":
		m.setKind(int(docDesign))
	case "3":
		m.setKind(int(docTasks))
	case "j":
		m.scrollBy(1)
	case "k":
		m.scrollBy(-1)
	case "ctrl+d":
		m.scrollBy(page / 2)
	case "ctrl+u":
		m.scrollBy(-page / 2)
	case "pgdown", "space", "ctrl+f":
		m.scrollBy(page)
	case "pgup", "ctrl+b":
		m.scrollBy(-page)
	case "g", "home":
		m.scrollTo(0)
	case "G", "end":
		m.scrollTo(1 << 30)
	case "r":
		m.list = loading
		return tea.Batch(specsCmd(m.be, m.ws.ID), m.reloadDoc())
	}
	return nil
}

// selectSpec moves the list selection and loads that spec.
func (m *model) selectSpec(i int) tea.Cmd {
	i = min(max(i, 0), len(m.specs)-1)
	m.sel = i
	return m.ensureDoc()
}

// setKind switches the document tab.
func (m *model) setKind(k int) { m.kind = docKind(k) }

// scrollBy scrolls the document; Render clamps to the document's length.
func (m *model) scrollBy(d int) {
	if s, ok := m.current(); ok {
		m.scroll[m.kind][s.Slug] = max(m.scroll[m.kind][s.Slug]+d, 0)
	}
}

// scrollTo sets the scroll position.
func (m *model) scrollTo(n int) {
	if s, ok := m.current(); ok {
		m.scroll[m.kind][s.Slug] = max(n, 0)
	}
}
