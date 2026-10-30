package codeview

import (
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
)

// onEdit handles an edit event (core.EditEvent) for this session. While
// following, it opens the edited file and scrolls to the edited lines,
// highlighting them. While paused it only refreshes the file on screen so it
// does not go stale; the newest edit is remembered for resuming.
func (m *model) onEdit(e core.EditEvent) tea.Cmd {
	e.Path = normPath(e.Path)
	m.last = &e
	if !m.follow || e.Path == "" {
		return nil
	}
	if m.paused {
		if e.Path == m.path && m.src != nil {
			return m.open(e.Path, 0, 0, 0, true)
		}
		return nil
	}
	return m.open(e.Path, 0, max(e.StartLine, 1), max(e.EndLine, e.StartLine), false)
}

// toggleFollow is the `f` key: start following, stop following, or, while
// paused, resume and jump to the newest edit. It only changes what this view
// shows and tells the daemon to keep the follow flag; it never sends
// anything to the agent.
func (m *model) toggleFollow() tea.Cmd {
	if m.sess == nil {
		return nil
	}
	if m.follow && m.paused {
		m.paused = false
		if m.last != nil && m.last.Path != "" {
			return m.open(m.last.Path, 0, max(m.last.StartLine, 1), max(m.last.EndLine, m.last.StartLine), false)
		}
		return nil
	}
	m.follow = !m.follow
	m.paused = false
	return followCmd(m.be, m.sess.ID, m.follow)
}

// pauseFollow is called by every user scroll or navigation: it pauses
// following and shows the "following paused" indicator.
func (m *model) pauseFollow() {
	if m.follow {
		m.paused = true
	}
	m.fresh.on = m.fresh.on && m.th.Animated() // a static marker ends at the first interaction
}
