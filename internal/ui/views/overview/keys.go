package overview

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// onKey routes a key press: to the ask/steer box while it is open, to the
// file links while they are selected, otherwise to the cards.
func (m *model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.sess == nil {
		return nil
	}
	if m.in != nil {
		return m.inputKey(msg)
	}
	if m.zone == zoneFiles {
		return m.fileKey(msg.String())
	}
	return m.cardKey(msg.String())
}

// cardKey handles keys while the card list has the selection.
func (m *model) cardKey(k string) tea.Cmd {
	switch k {
	case "up", "k":
		if m.sel == 0 && m.nFiles > 0 {
			m.markSeen(m.sel)
			m.zone, m.fileSel = zoneFiles, 0
			return nil
		}
		m.selectCard(m.sel - 1)
	case "down", "j":
		m.selectCard(m.sel + 1)
	case "pgup":
		m.selectCard(m.sel - 3)
	case "pgdown":
		m.selectCard(m.sel + 3)
	case "home", "g":
		m.selectCard(0)
	case "end", "G":
		m.selectCard(len(m.cards) - 1)
	case "f":
		if m.nFiles > 0 {
			m.markSeen(m.sel)
			m.zone, m.fileSel = zoneFiles, 0
		}
	case "a":
		m.openInput(core.NoteAsk)
	case "s":
		m.openInput(core.NoteSteer)
	case "i":
		m.openInput(core.NoteInterruptSteer)
	case "d":
		return m.dismiss()
	case "enter", "o":
		return m.openRef(m.sel, 0)
	default:
		if len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
			// A digit picks the card showing that number.
			for i, n := range m.cards {
				if cardNumber(n, i) == int(k[0]-'0') {
					m.selectCard(i)
					break
				}
			}
		}
	}
	return nil
}

// fileKey handles keys while a summary file link has the selection.
func (m *model) fileKey(k string) tea.Cmd {
	switch k {
	case "left", "h":
		m.fileSel = max(m.fileSel-1, 0)
	case "right", "l":
		m.fileSel = min(m.fileSel+1, max(m.nFiles-1, 0))
	case "down", "j", "esc":
		m.zone = zoneCards
	case "enter", "o":
		return m.openFile(m.fileSel)
	}
	return nil
}

// inputKey handles keys while the ask/steer box is open.
func (m *model) inputKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.in = nil
		return nil
	case "enter":
		text := strings.TrimSpace(m.in.ti.Value())
		if text == "" {
			return nil
		}
		p := protocol.NoteActParams{NoteID: m.in.noteID, Action: m.in.action, Text: text}
		n := m.in.number
		m.in = nil
		return actCmd(m.be, p, n)
	}
	var cmd tea.Cmd
	m.in.ti, cmd = m.in.ti.Update(msg)
	return cmd
}

// selectCard moves the card selection, clamped, marking the card left as seen.
func (m *model) selectCard(i int) {
	if len(m.cards) == 0 {
		return
	}
	i = min(max(i, 0), len(m.cards)-1)
	if i != m.sel {
		m.markSeen(m.sel)
	}
	m.sel = i
	m.zone = zoneCards
}

// markSeen records that the user has looked at card i.
func (m *model) markSeen(i int) {
	if i >= 0 && i < len(m.cards) {
		m.seen[m.cards[i].ID] = true
	}
}

// openInput opens the ask/steer box under the selected card.
func (m *model) openInput(a core.NoteAction) {
	if m.sel < 0 || m.sel >= len(m.cards) {
		return
	}
	m.in = newInput(m.th, a, m.cards[m.sel].ID, cardNumber(m.cards[m.sel], m.sel))
}

// dismiss hides the selected card at once and tells the daemon.
func (m *model) dismiss() tea.Cmd {
	if m.sel < 0 || m.sel >= len(m.cards) {
		return nil
	}
	n := m.cards[m.sel]
	m.dismissed[n.ID] = true
	m.rebuildCards()
	return actCmd(m.be, protocol.NoteActParams{NoteID: n.ID, Action: core.NoteDismiss}, cardNumber(n, m.sel))
}

// openRef opens reference r of card i in the Code view.
func (m *model) openRef(i, r int) tea.Cmd {
	if i < 0 || i >= len(m.cards) || r >= len(m.cards[i].Refs) {
		return nil
	}
	m.markSeen(i)
	path, line := parseRef(m.cards[i].Refs[r])
	if path == "" {
		return nil
	}
	return switchCmd(view.ModeCode, path, line)
}

// openFile opens summary file link i in the Diff view; the "+N more" link
// opens the Diff view without a file.
func (m *model) openFile(i int) tea.Cmd {
	if i < 0 || i >= m.nFiles {
		return nil
	}
	if i < m.nVisFiles && i < len(m.fileList) {
		return switchCmd(view.ModeDiff, m.fileList[i], 0)
	}
	return switchCmd(view.ModeDiff, "", 0)
}
