package codeview

import (
	"encoding/json"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/syntax"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// Update handles messages. It never does I/O; backend calls are returned as
// commands.
func (m *model) Update(msg tea.Msg) (view.View, tea.Cmd) {
	m.clock = m.now()
	m.decayFresh()
	switch msg := msg.(type) {
	case view.SessionMsg:
		return m, m.setSession(msg.Session)
	case view.FocusMsg:
		m.focused = msg.Focused
		return m, nil
	case view.TickMsg:
		m.frame = msg.Frame
		return m, nil
	case view.EventMsg:
		return m, m.onEvent(backend.Event(msg))
	case view.SwitchModeMsg:
		return m, m.onSwitch(msg)
	case fileMsg:
		return m, m.onFile(msg)
	case changesMsg:
		m.onChanges(msg)
		return m, nil
	case followErrMsg:
		return m, m.onFollowErr(msg)
	case syntax.ChunkMsg:
		if m.src != nil {
			m.src.Apply(msg)
		}
		return m, nil
	case tea.KeyPressMsg:
		return m, m.onKey(msg)
	case tea.MouseClickMsg:
		return m, m.onClick(msg)
	case tea.MouseWheelMsg:
		return m, m.onWheel(msg)
	default:
		if m.picker != nil {
			var cmd tea.Cmd
			m.picker.in, cmd = m.picker.in.Update(msg)
			m.picker.refilter(m.changes)
			return m, cmd
		}
	}
	return m, nil
}

// decayFresh clears the new-lines highlight once it has finished: when the
// decay is over (animated), or on the next interaction (static marker, which
// has no clock to fade by).
func (m *model) decayFresh() {
	if m.fresh.on && m.th.Animated() && m.clock.Sub(m.fresh.at) >= m.th.Motion.HunkDecay {
		m.fresh.on = false
	}
}

// setSession switches to another session (or none).
func (m *model) setSession(s *core.Session) tea.Cmd {
	if s == nil {
		m.reset()
		m.sess = nil
		return nil
	}
	if m.sess == nil || m.sess.ID != s.ID {
		m.reset()
	}
	c := *s
	m.sess = &c
	m.follow = s.Follow
	return m.loadChanges()
}

// reset drops everything tied to the previous session.
func (m *model) reset() {
	m.reqSeq++
	m.changes = nil
	m.chLoading, m.chDirty = false, false
	m.path, m.src, m.note, m.loading = "", nil, "", ""
	m.cursor, m.top = 0, 0
	m.fresh.on = false
	m.follow, m.paused = false, false
	m.last = nil
	m.picker = nil
}

// loadChanges fetches the list of changed files, coalescing bursts.
func (m *model) loadChanges() tea.Cmd {
	if m.sess == nil {
		return nil
	}
	if m.chLoading {
		m.chDirty = true
		return nil
	}
	m.chLoading = true
	return changesCmd(m.be, m.sess.ID)
}

// onChanges applies the changed-file list.
func (m *model) onChanges(msg changesMsg) {
	if m.sess == nil || msg.sess != m.sess.ID {
		return
	}
	m.chLoading = false
	if msg.err == nil {
		m.changes = msg.changes
		if m.picker != nil {
			m.picker.refilter(m.changes)
		}
	}
}

// onSwitch opens the file the shell asked for (a link from the Overview, a
// steering file from the sidebar). The user chose it, so it pauses following.
func (m *model) onSwitch(msg view.SwitchModeMsg) tea.Cmd {
	if msg.Mode != view.ModeCode || msg.Path == "" {
		return nil
	}
	m.picker = nil
	m.pauseFollow()
	return m.open(msg.Path, max(msg.Line, 0), 0, 0, false)
}

// onEvent reacts to daemon events about this session.
func (m *model) onEvent(ev backend.Event) tea.Cmd {
	if m.sess == nil {
		return nil
	}
	id := m.sess.ID
	switch ev.Topic {
	case protocol.TopicEdit:
		var e core.EditEvent
		if json.Unmarshal(ev.Data, &e) == nil && e.SessionID == id {
			return m.onEdit(e)
		}
	case protocol.TopicDiffChanged:
		var r protocol.SessionRef
		if json.Unmarshal(ev.Data, &r) == nil && r.SessionID == id {
			return m.loadChanges()
		}
	case protocol.TopicSession:
		var s core.Session
		if json.Unmarshal(ev.Data, &s) == nil && s.ID == id {
			c := s
			m.sess = &c
			if s.Follow != m.follow {
				m.follow = s.Follow
				m.paused = false
			}
		}
	}
	return nil
}

// onFollowErr undoes the optimistic follow toggle and says why.
func (m *model) onFollowErr(msg followErrMsg) tea.Cmd {
	if m.sess == nil || msg.sess != m.sess.ID {
		return nil
	}
	m.follow, m.paused = !msg.on, false
	body := clean(msg.err.Error(), true)
	return func() tea.Msg {
		return view.ToastMsg{Tone: kit.ToneDanger, Title: "Could not change follow mode", Body: body}
	}
}

// changeIndex returns the 0-based position of path among the changes, or -1.
func (m *model) changeIndex(path string) int {
	for i, c := range m.changes {
		if c.Path == path {
			return i
		}
	}
	return -1
}
