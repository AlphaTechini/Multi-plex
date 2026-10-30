package pane

import tea "charm.land/bubbletea/v2"

// Update handles the pane's own messages (Msg values). Frames and exits are
// applied with Apply and MarkExited by the owner, which decodes events once.
func (p *Pane) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case SnapshotMsg:
		return p.handleSnapshot(m)
	case resizeMsg:
		return p.handleResize(m)
	}
	return nil
}
