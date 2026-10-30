package app

import (
	"context"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// action is something the user can do: from the palette, or as a prefix key.
type action struct {
	group string
	title string
	// keys are the prefix-relative keys; the first one is shown.
	keys []string
	run  func(*Model) tea.Cmd
	// when reports whether the action applies right now (nil: always).
	when func(*Model) bool
}

// shortcut returns the display spec of the action's prefix key ("" if none).
func (a action) shortcut(prefix string) string {
	if len(a.keys) == 0 {
		return ""
	}
	return prefix + " " + a.keys[0]
}

func hasSession(m *Model) bool { return m.session() != nil }

// staticActions are the actions that do not depend on data.
var staticActions []action

func init() { staticActions = buildStaticActions() }

func buildStaticActions() []action {
	acts := []action{
		{group: "Sessions", title: "New session", keys: []string{"n"}, run: (*Model).actNewSession},
		{group: "Sessions", title: "Next session", keys: []string{"]"}, run: func(m *Model) tea.Cmd { return m.moveSession(1) }, when: hasSession},
		{group: "Sessions", title: "Previous session", keys: []string{"["}, run: func(m *Model) tea.Cmd { return m.moveSession(-1) }, when: hasSession},
		{group: "Sessions", title: "Stop session", keys: []string{"x"}, run: (*Model).actStop, when: func(m *Model) bool {
			s := m.session()
			return s != nil && s.State.Active()
		}},
		{group: "Sessions", title: "Restart session", keys: []string{"r"}, run: (*Model).actRestart, when: hasSession},
		{group: "Sessions", title: "Remove session", keys: []string{"X"}, run: (*Model).actRemove, when: hasSession},
		{group: "Sessions", title: "Toggle follow mode", keys: []string{"f"}, run: (*Model).actFollow, when: hasSession},
		{group: "Sessions", title: "Review pending approvals", keys: []string{"!"}, run: (*Model).actApprovals, when: func(m *Model) bool { return len(m.approvals) > 0 }},
	}
	for i, mode := range modeOrder {
		acts = append(acts, action{
			group: "View", title: "Switch to " + modeName(mode),
			keys: []string{itoa(i + 1), modeKeys[mode]},
			run:  func(m *Model) tea.Cmd { return m.actMode(mode) },
		})
	}
	return append(acts,
		action{group: "View", title: "Toggle agent panel", keys: []string{"a"}, run: (*Model).toggleAgent},
		action{group: "View", title: "Zoom focused region", keys: []string{"z"}, run: (*Model).toggleZoom},
		action{group: "View", title: "Swap agent panel side", keys: []string{"w"}, run: (*Model).swapAgentSide},
		action{group: "View", title: "Focus left", keys: []string{"h", "left"}, run: func(m *Model) tea.Cmd { return m.moveFocus(-1) }},
		action{group: "View", title: "Focus right", keys: []string{"l", "right"}, run: func(m *Model) tea.Cmd { return m.moveFocus(1) }},
		action{group: "App", title: "Command palette", keys: []string{"p"}, run: (*Model).actPalette},
		action{group: "App", title: "Show all keys", keys: []string{"?"}, run: (*Model).actHelp},
		action{group: "App", title: "Dismiss notifications", keys: []string{"."}, run: (*Model).actDismissToasts, when: func(m *Model) bool { return len(m.toasts) > 0 }},
		action{group: "App", title: "Detach and quit", keys: []string{"q"}, run: (*Model).actQuit},
	)
}

// paletteActions returns every action that applies now, including one entry
// per session, spec and steering file.
func (m *Model) paletteActions() []action {
	var out []action
	for _, a := range staticActions {
		if a.when == nil || a.when(m) {
			out = append(out, a)
		}
	}
	for _, s := range m.orderedSessions() {
		id := s.ID
		out = append(out, action{
			group: "Sessions", title: "Switch to session " + s.Slug,
			run: func(m *Model) tea.Cmd { return tea.Batch(m.selectSession(id), m.setFocus(regionMain)) },
		})
	}
	for _, sp := range m.specs {
		slug := sp.Slug
		out = append(out, action{
			group: "Specs", title: "Open spec " + slug,
			run: func(m *Model) tea.Cmd { return m.switchMode(view.SwitchModeMsg{Mode: view.ModeSpecs, Path: slug}) },
		})
	}
	for _, st := range m.info.Steering {
		st := st
		out = append(out, action{
			group: "Steering", title: "Open " + st.Label,
			run: func(m *Model) tea.Cmd { return m.openSteering(st) },
		})
	}
	// Group the entries, keeping the order inside each group.
	sort.SliceStable(out, func(i, j int) bool { return groupRank(out[i].group) < groupRank(out[j].group) })
	return out
}

// groupRank orders the palette's groups.
func groupRank(g string) int {
	switch g {
	case "Sessions":
		return 0
	case "View":
		return 1
	case "Specs":
		return 2
	case "Steering":
		return 3
	}
	return 4
}

// prefixAction finds the action bound to a prefix-relative key.
func (m *Model) prefixAction(key string) (action, bool) {
	for _, a := range staticActions {
		for _, k := range a.keys {
			if k == key {
				return a, a.when == nil || a.when(m)
			}
		}
	}
	return action{}, false
}

func (m *Model) openSteering(st steeringItem) tea.Cmd {
	return m.switchMode(view.SwitchModeMsg{Mode: view.ModeCode, Path: st.Path})
}

func (m *Model) actMode(mode view.Mode) tea.Cmd {
	return tea.Batch(m.setMode(mode), m.setFocus(regionMain))
}

func (m *Model) actPalette() tea.Cmd {
	m.pushOverlay(newPalette(m))
	return nil
}

func (m *Model) actHelp() tea.Cmd {
	m.pushOverlay(newHelp(m))
	return nil
}

func (m *Model) actNewSession() tea.Cmd {
	if m.notRepo {
		return m.addToast(toastSpec{tone: kit.ToneWarning, title: "No repository here", body: "Start mpx from inside a git repository to create sessions."})
	}
	o, cmd := newNewSession(m)
	m.pushOverlay(o)
	return cmd
}

func (m *Model) actDismissToasts() tea.Cmd {
	m.toasts = nil
	return nil
}

func (m *Model) actQuit() tea.Cmd {
	m.quit = true
	return tea.Quit
}

func (m *Model) actApprovals() tea.Cmd {
	if len(m.approvals) == 0 {
		return nil
	}
	m.showApproval(m.approvals[0].req.ID)
	return nil
}

// call runs a backend call off the loop and reports failure as a toast.
func (m *Model) call(what string, fn func(ctx context.Context) error) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := fn(ctx); err != nil {
			return errMsg{what: what, err: err}
		}
		return nil
	}
}

// errMsg reports a failed backend call.
type errMsg struct {
	what string
	err  error
}

func (m *Model) actStop() tea.Cmd {
	s := m.session()
	if s == nil {
		return nil
	}
	id, be := s.ID, m.be
	return tea.Batch(m.call("Stop "+s.Slug, func(ctx context.Context) error { return be.StopSession(ctx, id) }),
		m.addToast(toastSpec{tone: kit.ToneInfo, title: "Stopping " + s.Slug, body: "The worktree is kept"}))
}

func (m *Model) actRestart() tea.Cmd {
	s := m.session()
	if s == nil {
		return nil
	}
	id, be := s.ID, m.be
	return m.call("Restart "+s.Slug, func(ctx context.Context) error {
		_, err := be.RestartSession(ctx, id)
		return err
	})
}

func (m *Model) actFollow() tea.Cmd {
	s := m.session()
	if s == nil {
		return nil
	}
	id, be, on := s.ID, m.be, !s.Follow
	label := "off"
	if on {
		label = "on"
	}
	m.sessionByID(id).Follow = on
	return tea.Batch(m.call("Follow mode", func(ctx context.Context) error { return be.SetFollow(ctx, id, on) }),
		m.addToast(toastSpec{tone: kit.ToneInfo, title: "Follow mode " + label, body: s.Slug}))
}
