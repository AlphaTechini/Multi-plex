package overview

import (
	"encoding/json"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// Update handles messages. It never does I/O; backend calls are returned as
// commands.
func (m *model) Update(msg tea.Msg) (view.View, tea.Cmd) {
	m.clock = m.now()
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
	case loadedMsg:
		return m, m.onLoaded(msg)
	case tea.KeyPressMsg:
		return m, m.onKey(msg)
	case tea.MouseClickMsg:
		return m, m.onClick(msg)
	case tea.MouseWheelMsg:
		return m, m.onWheel(msg)
	default:
		if m.in != nil { // pastes, cursor messages
			var cmd tea.Cmd
			m.in.ti, cmd = m.in.ti.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// setSession switches to another session (or none) and starts a load.
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
	return m.startLoad(true)
}

// reset drops everything tied to the previous session.
func (m *model) reset() {
	m.loadSeq++
	m.loading, m.dirty = false, false
	m.detail, m.hasDetail = protocol.SessionDetail{}, false
	m.spec = nil
	m.cards = nil
	m.sel, m.top, m.zone, m.fileSel = 0, 0, zoneCards, 0
	m.in = nil
	m.checkedAt = time.Time{}
	m.loadErr = ""
	m.nFiles, m.nVisFiles, m.fileList = 0, 0, nil
}

// startLoad fetches the session detail, or marks the view dirty when a load
// is already in flight so it reloads once more when that one lands.
func (m *model) startLoad(withSpecs bool) tea.Cmd {
	if m.sess == nil {
		return nil
	}
	if m.loading {
		m.dirty = true
		m.dirtySpecs = m.dirtySpecs || withSpecs
		return nil
	}
	m.loading = true
	m.loadSeq++
	return loadCmd(m.be, m.ws.ID, m.sess.ID, m.loadSeq, withSpecs)
}

// onLoaded applies a finished load.
func (m *model) onLoaded(msg loadedMsg) tea.Cmd {
	if m.sess == nil || msg.id != m.sess.ID || msg.seq != m.loadSeq {
		return nil
	}
	m.loading = false
	if msg.err != nil {
		m.loadErr = msg.err.Error()
	} else {
		m.loadErr = ""
		m.detail, m.hasDetail = msg.detail, true
		for id, name := range msg.names {
			m.names[id] = name
		}
		if !msg.keep {
			m.spec = msg.spec
		}
		if m.checkedAt.IsZero() {
			m.checkedAt = msg.at
		}
		m.rebuildCards()
	}
	if m.dirty {
		withSpecs := m.dirtySpecs
		m.dirty, m.dirtySpecs = false, false
		return m.startLoad(withSpecs)
	}
	return nil
}

// rebuildCards recomputes the visible card list (undismissed, newest first)
// and keeps the selection on the same card where possible.
func (m *model) rebuildCards() {
	var keep string
	if m.sel >= 0 && m.sel < len(m.cards) {
		keep = m.cards[m.sel].ID
	}
	cards := m.cards[:0:0]
	for _, n := range m.detail.Notes {
		if n.Dismissed || m.dismissed[n.ID] {
			continue
		}
		cards = append(cards, n)
	}
	sort.SliceStable(cards, func(i, j int) bool {
		if !cards[i].At.Equal(cards[j].At) {
			return cards[i].At.After(cards[j].At)
		}
		return cards[i].Number > cards[j].Number
	})
	m.cards = cards
	m.sel = min(max(m.sel, 0), max(len(cards)-1, 0))
	if keep != "" {
		for i, n := range cards {
			if n.ID == keep {
				m.sel = i
				break
			}
		}
	}
	if m.in != nil && (m.sel >= len(cards) || cards[m.sel].ID != m.in.noteID) {
		m.in = nil // the card went away under the box
	}
}

// onEvent reloads when the daemon reports news about this session.
func (m *model) onEvent(ev backend.Event) tea.Cmd {
	if m.sess == nil {
		return nil
	}
	id := m.sess.ID
	switch ev.Topic {
	case protocol.TopicNote:
		var n core.Note
		if json.Unmarshal(ev.Data, &n) == nil && n.SessionID == id {
			return m.startLoad(false)
		}
	case protocol.TopicNoteDismissed:
		var n core.Note
		if json.Unmarshal(ev.Data, &n) == nil && n.SessionID == id {
			m.dismissed[n.ID] = true
			m.rebuildCards()
			return m.startLoad(false)
		}
	case protocol.TopicSummary:
		var s core.Summary
		if json.Unmarshal(ev.Data, &s) == nil && s.SessionID == id {
			return m.startLoad(false)
		}
	case protocol.TopicConflicts:
		var c protocol.ConflictsEvent
		if json.Unmarshal(ev.Data, &c) == nil && c.SessionID == id {
			m.detail.Conflicts = c.Conflicts
			m.checkedAt = m.clock
			return m.startLoad(false)
		}
	case protocol.TopicSession:
		var s core.Session
		if json.Unmarshal(ev.Data, &s) == nil && s.ID == id {
			changed := s.State != m.sess.State
			c := s
			m.sess = &c
			m.detail.Session = s
			if changed {
				return m.startLoad(true) // approvals and checkpoints move with state
			}
		}
	}
	return nil
}
