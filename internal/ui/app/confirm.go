package app

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// confirmOverlay asks the user to confirm something that cannot be undone.
// Enter confirms and Esc cancels; nothing else does.
type confirmOverlay struct {
	title   string
	lines   []string
	confirm string // the confirm button's label
	run     func(*Model) tea.Cmd
}

func (c *confirmOverlay) update(m *Model, msg tea.Msg) (overlay, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return c, nil
	}
	switch k.String() {
	case "enter":
		return nil, c.run(m)
	case "esc", "n":
		return nil, nil
	}
	return c, nil
}

func (c *confirmOverlay) view(m *Model, screen string) string {
	t := m.th
	mo := kit.ModalOptions{
		Width: 70, Tone: kit.ToneWarning, Title: c.title,
		Buttons: []kit.ButtonOptions{
			{Label: c.confirm, Keys: "enter", Variant: kit.ButtonPrimary},
			{Label: "Cancel", Keys: "esc", Variant: kit.ButtonGhost},
		},
		Screen: screen, ScreenWidth: m.w, ScreenHeight: m.h, Top: -1,
	}
	w := kit.ModalInnerWidth(mo)
	var out []string
	for i, l := range c.lines {
		if i > 0 {
			out = append(out, "")
		}
		for _, wl := range kit.Wrap(l, w) {
			out = append(out, kit.Fit(paint(t.Text.Secondary, false, wl), w))
		}
	}
	mo.Body = strings.Join(out, "\n")
	return kit.Modal(t, mo)
}

// actRemove asks to remove the selected session with its worktree and branch.
func (m *Model) actRemove() tea.Cmd {
	s := m.session()
	if s == nil {
		return nil
	}
	id, slug, branch := s.ID, s.Slug, s.Branch
	m.pushOverlay(&confirmOverlay{
		title:   "Remove " + slug + "?",
		confirm: "Remove session",
		lines: []string{
			"This stops the agent and deletes its worktree and the branch " + branch + ".",
			"Uncommitted changes and commits that are not merged anywhere else are lost.",
		},
		run: func(m *Model) tea.Cmd { return m.removeCmd(id, slug) },
	})
	return nil
}

// removeCmd removes a session off the loop; the session.removed event updates
// the screen.
func (m *Model) removeCmd(id core.SessionID, slug string) tea.Cmd {
	be := m.be
	return tea.Batch(
		func() tea.Msg {
			// Removing a worktree can take a while (it retries on Windows).
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			if err := be.RemoveSession(ctx, id); err != nil {
				return errMsg{what: "Remove " + slug, err: err}
			}
			return nil
		},
		m.addToast(toastSpec{tone: kit.ToneInfo, title: "Removing " + slug, body: "Worktree and branch are deleted"}),
	)
}
