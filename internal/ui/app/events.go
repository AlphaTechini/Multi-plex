package app

import (
	"encoding/json"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// eventsMsg is a batch of daemon events.
type eventsMsg struct {
	events []backend.Event
	closed bool
}

// maxBatch bounds how many queued events one message carries, so a flood of
// pane frames costs one render, not one per frame.
const maxBatch = 128

// waitEvents returns the command that waits for the next daemon events. The
// shell re-arms it after each batch.
func (m *Model) waitEvents() tea.Cmd {
	ch := m.be.Events()
	if ch == nil || m.noPump {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return eventsMsg{closed: true}
		}
		out := eventsMsg{events: []backend.Event{ev}}
		for len(out.events) < maxBatch {
			select {
			case ev, ok := <-ch:
				if !ok {
					out.closed = true
					return out
				}
				out.events = append(out.events, ev)
			default:
				return out
			}
		}
		return out
	}
}

// handleEvents routes a batch: the shell reacts to what concerns it, then
// every built view gets the event too.
func (m *Model) handleEvents(b eventsMsg) tea.Cmd {
	var cmds []tea.Cmd
	for _, ev := range b.events {
		cmds = append(cmds, m.handleEvent(ev))
		cmds = append(cmds, m.forwardEvent(ev))
	}
	if b.closed {
		cmds = append(cmds, m.addToast(toastSpec{tone: kit.ToneDanger, title: "Disconnected from the daemon", body: "Sessions keep running. Restart mpx to reattach.", sticky: true}))
	} else {
		cmds = append(cmds, m.waitEvents())
	}
	return tea.Batch(cmds...)
}

// handleEvent applies one event to the shell's own state.
func (m *Model) handleEvent(ev backend.Event) tea.Cmd {
	switch ev.Topic {
	case protocol.TopicPaneFrame:
		if len(m.agentPanes) == 0 {
			return nil
		}
		var f protocol.PaneFrame
		if json.Unmarshal(ev.Data, &f) == nil {
			if p := m.agentPanes[f.PaneID]; p != nil {
				p.Apply(f)
			}
		}
	case protocol.TopicPaneExited:
		var info core.Pane
		if json.Unmarshal(ev.Data, &info) == nil {
			if p := m.agentPanes[info.ID]; p != nil {
				p.MarkExited(info)
			}
		}
	case protocol.TopicSession:
		var s core.Session
		if json.Unmarshal(ev.Data, &s) == nil {
			return m.handleSessionUpdated(s)
		}
	case protocol.TopicSessionRemoved:
		var ref protocol.SessionRef
		if json.Unmarshal(ev.Data, &ref) == nil {
			return m.removeSession(ref.SessionID)
		}
	case protocol.TopicNote:
		var n core.Note
		if json.Unmarshal(ev.Data, &n) == nil && !n.Dismissed {
			m.notes[n.SessionID]++
		}
	case protocol.TopicNoteDismissed:
		var n core.Note
		if json.Unmarshal(ev.Data, &n) == nil && m.notes[n.SessionID] > 0 {
			m.notes[n.SessionID]--
		}
	case protocol.TopicApprovalPending:
		var p protocol.ApprovalPending
		if json.Unmarshal(ev.Data, &p) == nil {
			return m.handleApprovalPending(p)
		}
	case protocol.TopicApprovalDone:
		var d core.ApprovalDecision
		if json.Unmarshal(ev.Data, &d) == nil {
			m.resolveApproval(d.RequestID)
		}
	case protocol.TopicToast:
		var t protocol.Toast
		if json.Unmarshal(ev.Data, &t) == nil {
			return m.addToast(toastSpec{tone: toastTone(t.Level), title: t.Text})
		}
	}
	return nil
}

func toastTone(level string) kit.Tone {
	switch level {
	case "success":
		return kit.ToneSuccess
	case "warning":
		return kit.ToneWarning
	case "error":
		return kit.ToneDanger
	}
	return kit.ToneInfo
}

// handleSessionUpdated upserts a session, announces important transitions and
// selects a session the user just created.
func (m *Model) handleSessionUpdated(s core.Session) tea.Cmd {
	old, existed := m.upsertSession(s)
	var cmds []tea.Cmd
	if existed && old.State != s.State {
		switch s.State {
		case core.StateDone:
			cmds = append(cmds, m.addToast(toastSpec{tone: kit.ToneSuccess, title: s.Slug + " is done", body: "Ready for review"}))
		case core.StateError:
			cmds = append(cmds, m.addToast(toastSpec{tone: kit.ToneDanger, title: s.Slug + " failed", body: s.StateDetail}))
		}
	}
	if s.ID == m.cur {
		cmds = append(cmds, m.ensureAgent(&s))
		m.pruneAgentPanes()
	}
	if m.pendingSelect != "" && s.Slug == m.pendingSelect {
		m.pendingSelect = ""
		cmds = append(cmds, m.selectSession(s.ID), m.setFocus(regionMain))
	} else if m.cur == "" && !existed {
		cmds = append(cmds, m.selectSession(s.ID))
	}
	return tea.Batch(cmds...)
}

// pruneAgentPanes drops pane components no session refers to any more.
func (m *Model) pruneAgentPanes() {
	for id := range m.agentPanes {
		found := false
		for _, s := range m.sessions {
			if s.AgentPane == id {
				found = true
				break
			}
		}
		if !found {
			delete(m.agentPanes, id)
		}
	}
}

// paneWatcher is implemented by views that draw a daemon pane (Term): they
// are sent pane.frame and pane.exited events. Other views never use them, and
// skipping them keeps a streaming agent from re-rendering the main view.
type paneWatcher interface{ WantsPaneFrames() bool }

// forwardEvent sends an event to the views: every event goes to every built
// view, except the high-rate pane topics, which go only to views that draw a
// pane.
func (m *Model) forwardEvent(ev backend.Event) tea.Cmd {
	if ev.Topic != protocol.TopicPaneFrame && ev.Topic != protocol.TopicPaneExited {
		return m.broadcast(view.EventMsg(ev))
	}
	var cmds []tea.Cmd
	for _, mode := range modeOrder {
		if w, ok := m.views[mode].(paneWatcher); ok && w.WantsPaneFrames() {
			cmds = append(cmds, m.sendView(mode, view.EventMsg(ev)))
		}
	}
	return tea.Batch(cmds...)
}
