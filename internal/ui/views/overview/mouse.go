package overview

import (
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
)

// onClick handles a left click using the rectangles of the last Render.
func (m *model) onClick(msg tea.MouseClickMsg) tea.Cmd {
	mm := msg.Mouse()
	if m.sess == nil || mm.Button != tea.MouseLeft {
		return nil
	}
	h, ok := hitAt(m.hits, mm.X, mm.Y)
	if !ok {
		return nil
	}
	if m.in != nil && h.kind != hitCard && h.kind != hitAction {
		m.in = nil
	}
	switch h.kind {
	case hitCard:
		m.selectCard(h.card)
	case hitRef:
		m.selectCard(h.card)
		return m.openRef(h.card, h.file)
	case hitAction:
		m.selectCard(h.card)
		m.in = nil
		switch h.action {
		case core.NoteDismiss:
			return m.dismiss()
		default:
			m.openInput(h.action)
		}
	case hitFile, hitMoreFiles:
		m.zone, m.fileSel = zoneFiles, h.file
		return m.openFile(h.file)
	case hitEarlier, hitLater:
		m.selectCard(h.card)
	}
	return nil
}

// onWheel scrolls the card selection.
func (m *model) onWheel(msg tea.MouseWheelMsg) tea.Cmd {
	if m.sess == nil || m.in != nil {
		return nil
	}
	switch msg.Mouse().Button {
	case tea.MouseWheelUp:
		m.selectCard(m.sel - 1)
	case tea.MouseWheelDown:
		m.selectCard(m.sel + 1)
	}
	return nil
}
