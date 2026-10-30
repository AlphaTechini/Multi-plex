package diffview

import (
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
)

// onSession switches the view to another session (or to none).
func (m *Model) onSession(s *core.Session) tea.Cmd {
	if s == nil {
		m.resetSession()
		m.session = nil
		return nil
	}
	if m.session != nil && m.session.ID == s.ID {
		cp := *s
		m.session = &cp
		return nil
	}
	m.resetSession()
	cp := *s
	m.session = &cp
	m.mem = m.mems[s.ID]
	if m.mem == nil {
		m.mem = &sessionMem{rev: newReviewState(), positions: map[string]posState{}}
		m.mems[s.ID] = m.mem
	}
	m.rev = m.mem.rev
	return tea.Batch(m.loadDiffCmd(), m.commentsCmd())
}

// resetSession forgets everything about the current session's diff. Reviewed
// marks and the last file are kept in the session's memory.
func (m *Model) resetSession() {
	m.leaveFile()
	m.gen++
	m.loaded, m.loading, m.debouncing, m.reloadAgain = false, false, false, false
	m.loadErr, m.base, m.head = "", "", ""
	m.files, m.fi, m.fv = nil, -1, nil
	m.filesVer++
	m.comments, m.cmp, m.saves = nil, nil, nil
	m.hlBusy = false
	m.overlay = false
	m.hx, m.railTop = 0, 0
	m.anim = animState{fadeStep: m.t.Motion.FadeSteps, settle: 1}
	m.initMaps()
	m.mem, m.rev = nil, nil
}
