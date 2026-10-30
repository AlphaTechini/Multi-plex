package diffview

import (
	"encoding/json"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// Update handles messages. It never does I/O: backend calls and highlighting
// run in the commands it returns.
func (m *Model) Update(msg tea.Msg) (view.View, tea.Cmd) {
	var cmd tea.Cmd
	animTick := false
	switch msg := msg.(type) {
	case view.SessionMsg:
		cmd = m.onSession(msg.Session)
	case view.FocusMsg:
		m.focused = msg.Focused
		if m.fv != nil {
			m.fv.gen++
		}
	case view.EventMsg:
		cmd = m.onEvent(msg)
	case view.TickMsg:
		animTick = true
		m.advanceAnim(m.now())
	case view.SwitchModeMsg:
		cmd = m.onOpen(msg)
	case tea.KeyPressMsg:
		m.finishFade()
		cmd = m.onKey(msg)
	case tea.PasteMsg:
		if m.cmp != nil {
			cmd = m.composePaste(msg)
		}
	case tea.MouseClickMsg:
		m.finishFade()
		cmd = m.onClick(msg)
	case tea.MouseMotionMsg:
		cmd = m.onDrag(msg)
	case tea.MouseReleaseMsg:
		m.dragging = false
	case tea.MouseWheelMsg:
		cmd = m.onWheel(msg)
	case refreshDueMsg:
		cmd = m.onRefreshDue(msg)
	case diffLoadedMsg:
		cmd = m.onDiffLoaded(msg)
	case contentMsg:
		cmd = m.onContent(msg)
	case commentsMsg:
		cmd = m.onComments(msg)
	case commentSavedMsg:
		cmd = m.onCommentSaved(msg)
	case commentDeletedMsg:
		cmd = m.onCommentDeleted(msg)
	case sentMsg:
		cmd = m.onSent(msg)
	case hlDoneMsg:
		cmd = m.onHighlight(msg)
	}
	if animTick {
		return m, cmd
	}
	return m, tea.Batch(cmd, m.highlightCmd())
}

// onEvent reacts to daemon events: a diff change in the current session
// schedules a debounced refresh.
func (m *Model) onEvent(ev view.EventMsg) tea.Cmd {
	if ev.Topic != protocol.TopicDiffChanged || m.session == nil {
		return nil
	}
	var ref protocol.SessionRef
	if err := json.Unmarshal(ev.Data, &ref); err == nil && ref.SessionID != "" && ref.SessionID != m.session.ID {
		return nil
	}
	return m.scheduleRefresh()
}

// scheduleRefresh starts the debounce window unless one is running.
func (m *Model) scheduleRefresh() tea.Cmd {
	if m.debouncing {
		return nil
	}
	m.debouncing = true
	gen := m.gen
	return tea.Tick(refreshDebounce, func(_ time.Time) tea.Msg { return refreshDueMsg{gen: gen} })
}

// onRefreshDue loads the diff when the debounce window ends.
func (m *Model) onRefreshDue(msg refreshDueMsg) tea.Cmd {
	if msg.gen != m.gen {
		return nil
	}
	m.debouncing = false
	if m.loading {
		m.reloadAgain = true
		return nil
	}
	return m.loadDiffCmd()
}

// toastErr is a danger toast for a failed backend call.
func toastErr(title string, err error) tea.Cmd {
	return toastCmd(kit.ToneDanger, title, err.Error())
}
