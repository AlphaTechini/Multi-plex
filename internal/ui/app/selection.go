package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/pane"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// sendView delivers a message to a built view and keeps the view it returns.
func (m *Model) sendView(mode view.Mode, msg tea.Msg) tea.Cmd {
	v, ok := m.views[mode]
	if !ok {
		return nil
	}
	nv, cmd := v.Update(msg)
	m.views[mode] = nv
	if mode == m.mode {
		m.mainVer++
	}
	return cmd
}

// broadcast forwards a message to every built view.
func (m *Model) broadcast(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for _, mode := range modeOrder {
		if c := m.sendView(mode, msg); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

// activateMode builds the mode's view if needed and tells it which session it
// belongs to (when that changed since it was last told), its size and its
// focus.
func (m *Model) activateMode(mode view.Mode) tea.Cmd {
	if !m.loaded {
		return nil
	}
	m.viewOf(mode)
	var cmds []tea.Cmd
	// A view built after the last resize has not been told its size (announce
	// only reports changes); the Term view opens its shell at that size.
	if cols, rows := m.lastMain[0], m.lastMain[1]; cols > 0 && rows > 0 {
		cmds = append(cmds, m.sendView(mode, tea.WindowSizeMsg{Width: cols, Height: rows}))
	}
	if told, ok := m.told[mode]; !ok || told != m.cur {
		m.told[mode] = m.cur
		var sel *core.Session
		if s := m.session(); s != nil {
			cp := *s
			sel = &cp
		}
		cmds = append(cmds, m.sendView(mode, view.SessionMsg{Session: sel}))
	}
	cmds = append(cmds, m.announce(), m.syncViewFocus())
	return tea.Batch(cmds...)
}

// syncViewFocus tells each view whether it has keyboard focus.
func (m *Model) syncViewFocus() tea.Cmd {
	var cmds []tea.Cmd
	for _, mode := range modeOrder {
		if _, ok := m.views[mode]; !ok {
			continue
		}
		want := m.focus == regionMain && mode == m.mode
		if m.viewFocus[mode] != want {
			m.viewFocus[mode] = want
			cmds = append(cmds, m.sendView(mode, view.FocusMsg{Focused: want}))
		}
	}
	return tea.Batch(cmds...)
}

// setMode switches the main view to a mode.
func (m *Model) setMode(mode view.Mode) tea.Cmd {
	if mode == m.mode || !m.loaded && mode != view.ModeOverview {
		return nil
	}
	m.mode = mode
	m.recompute()
	return m.activateMode(mode)
}

// switchMode handles view.SwitchModeMsg: show the mode, then let it open the
// file or line the message names.
func (m *Model) switchMode(msg view.SwitchModeMsg) tea.Cmd {
	cmds := []tea.Cmd{m.setMode(msg.Mode)}
	if m.focus != regionMain {
		cmds = append(cmds, m.setFocus(regionMain))
	}
	cmds = append(cmds, m.sendView(msg.Mode, msg))
	return tea.Batch(cmds...)
}

// selectSession makes id the session the main view and agent panel show.
func (m *Model) selectSession(id core.SessionID) tea.Cmd {
	s := m.sessionByID(id)
	if s == nil {
		return nil
	}
	m.cur = id
	m.sb.cursor = m.sbIndexOfSession(id)
	var cmds []tea.Cmd
	cmds = append(cmds, m.ensureAgent(s))
	if p := m.agentPaneOf(s); p != nil && (m.lay.drawer || !m.lay.agentRail) {
		// A pane that was not on screen may have missed a resize.
		if cols, rows := m.lay.agentInner(); cols > 0 && rows > 0 {
			cmds = append(cmds, p.Resize(cols, rows))
		}
	}
	cmds = append(cmds, m.activateMode(m.mode))
	if s.Unseen {
		cmds = append(cmds, m.markSeen(id))
	}
	m.sbEnsureVisible()
	return tea.Batch(cmds...)
}

// markSeen clears the daemon's unseen flag; the session.updated event that
// follows updates the sidebar.
func (m *Model) markSeen(id core.SessionID) tea.Cmd {
	be := m.be
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = be.MarkSeen(ctx, id)
		return nil
	}
}

// ensureAgent creates and attaches the pane component of a session's agent.
func (m *Model) ensureAgent(s *core.Session) tea.Cmd {
	if s == nil || s.AgentPane == "" || m.agentPanes[s.AgentPane] != nil {
		return nil
	}
	p := pane.New(pane.Options{Theme: m.th, Backend: m.be, Name: "agent"})
	m.agentPanes[s.AgentPane] = p
	cmds := []tea.Cmd{p.Attach(s.AgentPane)}
	if cols, rows := m.lay.agentInner(); cols > 0 && rows > 0 && m.lay.agentW > 0 {
		cmds = append(cmds, p.Resize(cols, rows))
	}
	return tea.Batch(cmds...)
}

// moveSession selects the next (delta 1) or previous (-1) session in sidebar
// order, wrapping around.
func (m *Model) moveSession(delta int) tea.Cmd {
	ss := m.orderedSessions()
	if len(ss) == 0 {
		return nil
	}
	i := 0
	for j, s := range ss {
		if s.ID == m.cur {
			i = j
		}
	}
	i = (i + delta + len(ss)) % len(ss)
	return m.selectSession(ss[i].ID)
}

// setSessions replaces the session list (initial load).
func (m *Model) setSessions(list []core.Session) {
	m.sessions = list
	m.sbRebuild()
	if m.cur != "" && m.sessionByID(m.cur) == nil {
		m.cur = ""
	}
}

// upsertSession inserts or updates a session and reports its previous state.
func (m *Model) upsertSession(s core.Session) (old core.Session, existed bool) {
	for i := range m.sessions {
		if m.sessions[i].ID == s.ID {
			old, m.sessions[i] = m.sessions[i], s
			return old, true
		}
	}
	m.sessions = append(m.sessions, s)
	m.sbRebuild()
	return core.Session{}, false
}

// removeSession drops a session and moves the selection off it.
func (m *Model) removeSession(id core.SessionID) tea.Cmd {
	for i := range m.sessions {
		if m.sessions[i].ID != id {
			continue
		}
		if p := m.sessions[i].AgentPane; p != "" {
			delete(m.agentPanes, p)
		}
		m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
		break
	}
	delete(m.notes, id)
	m.sbRebuild()
	if m.cur == id {
		m.cur = ""
		m.told = map[view.Mode]core.SessionID{}
		if len(m.sessions) > 0 {
			return m.selectSession(m.orderedSessions()[0].ID)
		}
		return m.activateMode(m.mode)
	}
	return nil
}
