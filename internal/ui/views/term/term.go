// Package term is the Term main-view mode: a shell in the selected session's
// worktree, hosted by the daemon and drawn with the shared pane component.
// The shell is opened lazily, the first time the mode is shown for a session,
// and kept while the session exists.
package term

import (
	"context"
	"encoding/json"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/pane"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// View is the Term mode.
type View struct {
	env view.Env

	session *core.Session
	panes   map[core.SessionID]*pane.Pane
	opening map[core.SessionID]bool
	errs    map[core.SessionID]string

	focused bool
	w, h    int
}

// New creates the Term view.
func New(env view.Env) view.View {
	return &View{
		env:     env,
		panes:   map[core.SessionID]*pane.Pane{},
		opening: map[core.SessionID]bool{},
		errs:    map[core.SessionID]string{},
	}
}

// openedMsg carries the result of OpenShell.
type openedMsg struct {
	id   core.SessionID
	info core.Pane
	err  error
}

// Tab returns the tab label.
func (v *View) Tab() (string, int) { return "Term", 0 }

// Hints returns no hints of its own: the shell adds the prefix hints.
func (v *View) Hints() []kit.Hint { return nil }

// WantsPaneFrames tells the shell to send this view pane.frame and
// pane.exited events, which it otherwise sends to no view.
func (v *View) WantsPaneFrames() bool { return true }

// Animating is always false: the terminal repaints from frame events.
func (v *View) Animating() bool { return false }

// Capturing is true while focused: every key goes to the shell.
func (v *View) Capturing() bool { return v.focused }

// current returns the pane of the selected session.
func (v *View) current() *pane.Pane {
	if v.session == nil {
		return nil
	}
	return v.panes[v.session.ID]
}

// Update handles view messages, daemon events and, while focused, keys.
func (v *View) Update(msg tea.Msg) (view.View, tea.Cmd) {
	switch m := msg.(type) {
	case view.SessionMsg:
		return v, v.setSession(m.Session)
	case tea.WindowSizeMsg:
		v.w, v.h = m.Width, m.Height
		var cmds []tea.Cmd
		for _, p := range v.panes {
			cmds = append(cmds, p.Resize(v.w, v.h))
		}
		return v, tea.Batch(cmds...)
	case view.FocusMsg:
		v.focused = m.Focused
	case openedMsg:
		return v, v.opened(m)
	case view.EventMsg:
		v.event(m.Topic, m.Data)
	case tea.KeyPressMsg:
		return v, v.key(m)
	case tea.PasteMsg:
		if p := v.current(); p != nil && v.focused {
			p.Paste(m.Content)
		}
	}
	return v, nil
}

// setSession switches the selected session and opens its shell on first show.
func (v *View) setSession(s *core.Session) tea.Cmd {
	v.session = s
	if s == nil || v.panes[s.ID] != nil || v.opening[s.ID] {
		return nil
	}
	return v.open(s.ID)
}

// open asks the daemon for a shell in the session's worktree.
func (v *View) open(id core.SessionID) tea.Cmd {
	v.opening[id] = true
	delete(v.errs, id)
	be := v.env.Backend
	cols, rows := max(v.w, 20), max(v.h, 5)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		info, err := be.OpenShell(ctx, id, cols, rows)
		return openedMsg{id: id, info: info, err: err}
	}
}

func (v *View) opened(m openedMsg) tea.Cmd {
	delete(v.opening, m.id)
	if m.err != nil {
		v.errs[m.id] = m.err.Error()
		return nil
	}
	p := pane.New(pane.Options{Theme: v.env.Theme, Backend: v.env.Backend, Name: "shell"})
	v.panes[m.id] = p
	return tea.Batch(p.Attach(m.info.ID), p.Resize(v.w, v.h))
}

// event applies pane frames and exits to the view's panes.
func (v *View) event(topic string, data json.RawMessage) {
	switch topic {
	case protocol.TopicPaneFrame:
		if len(v.panes) == 0 {
			return
		}
		var f protocol.PaneFrame
		if json.Unmarshal(data, &f) != nil {
			return
		}
		for _, p := range v.panes {
			p.Apply(f)
		}
	case protocol.TopicPaneExited:
		var info core.Pane
		if json.Unmarshal(data, &info) != nil {
			return
		}
		for _, p := range v.panes {
			p.MarkExited(info)
		}
	case protocol.TopicSessionRemoved:
		var ref protocol.SessionRef
		if json.Unmarshal(data, &ref) == nil {
			delete(v.panes, ref.SessionID)
		}
	}
}

// key sends a key to the shell, or restarts an exited shell on "r".
func (v *View) key(k tea.KeyPressMsg) tea.Cmd {
	if !v.focused || v.session == nil {
		return nil
	}
	p := v.current()
	if p == nil {
		return nil
	}
	if p.Key(k) {
		return nil
	}
	if exited, _ := p.Exited(); exited && k.String() == "r" {
		delete(v.panes, v.session.ID)
		return v.open(v.session.ID)
	}
	return nil
}

// Render draws the shell, or a short status when there is none yet.
func (v *View) Render(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	t := v.env.Theme
	p := v.current()
	switch {
	case p != nil:
		return p.Render(w, h, v.focused)
	case v.session == nil:
		return kit.EmptyState(t, kit.EmptyStateOptions{Width: w, Height: h, Title: "No session selected", Body: "Pick a session in the sidebar to get a shell in its worktree."})
	case v.errs[v.session.ID] != "":
		return kit.EmptyState(t, kit.EmptyStateOptions{Width: w, Height: h, Title: "Could not open a shell", Body: v.errs[v.session.ID]})
	default:
		return kit.EmptyState(t, kit.EmptyStateOptions{Width: w, Height: h, Title: "Opening a shell…", Body: "In " + v.session.WorktreePath})
	}
}
