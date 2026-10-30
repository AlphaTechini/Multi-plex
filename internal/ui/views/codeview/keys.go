package codeview

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// onKey routes a key press.
func (m *model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.picker != nil {
		return m.pickerKey(msg)
	}
	if m.sess == nil {
		return nil
	}
	k := msg.String()
	half := max(m.rows/2, 1)
	switch k {
	case "f":
		return m.toggleFollow()
	case "p", "ctrl+p", "/":
		m.picker = newPicker(m.th, m.changes)
		return nil
	case "e":
		return m.editorToast()
	case "]":
		return m.stepChange(1)
	case "[":
		return m.stepChange(-1)
	}
	if m.src == nil {
		return nil
	}
	switch k {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "ctrl+d":
		m.pageBy(half)
	case "ctrl+u":
		m.pageBy(-half)
	case "pgdown", "ctrl+f", "space":
		m.pageBy(m.rows)
	case "pgup", "ctrl+b":
		m.pageBy(-m.rows)
	case "g", "home":
		m.cursor = 0
		m.clampScroll()
	case "G", "end":
		m.cursor = m.lines() - 1
		m.clampScroll()
	default:
		return nil
	}
	m.pauseFollow()
	return m.request()
}

// stepChange opens the next or previous changed file.
func (m *model) stepChange(d int) tea.Cmd {
	if len(m.changes) == 0 {
		return nil
	}
	i := m.changeIndex(m.path)
	switch {
	case i < 0 && d > 0:
		i = 0
	case i < 0:
		i = len(m.changes) - 1
	default:
		i = (i + d + len(m.changes)) % len(m.changes)
	}
	m.pauseFollow()
	return m.open(m.changes[i].Path, 0, 0, 0, false)
}

// pickerKey handles keys while the go-to-file box is open.
func (m *model) pickerKey(msg tea.KeyPressMsg) tea.Cmd {
	p := m.picker
	switch msg.String() {
	case "esc":
		m.picker = nil
		return nil
	case "enter":
		it, ok := p.selected()
		m.picker = nil
		if !ok || strings.TrimSpace(it.path) == "" {
			return nil
		}
		m.pauseFollow()
		return m.open(it.path, 0, 0, 0, false)
	case "down", "ctrl+n", "tab":
		p.move(1)
		return nil
	case "up", "ctrl+p", "shift+tab":
		p.move(-1)
		return nil
	case "pgdown":
		p.move(pickerRows)
		return nil
	case "pgup":
		p.move(-pickerRows)
		return nil
	}
	var cmd tea.Cmd
	before := p.in.Value()
	p.in, cmd = p.in.Update(msg)
	if p.in.Value() != before {
		p.refilter(m.changes)
	}
	return cmd
}
