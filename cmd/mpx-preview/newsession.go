package main

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// newSession is the new-session dialog (docs/05: task, role, base branch)
// over the Overview screen.
func (s scene) newSession() string {
	t := s.t
	mo := kit.ModalOptions{
		Width: 84,
		Title: "New session", Meta: "multi-plex",
		Buttons: []kit.ButtonOptions{
			{Label: "Start session", Keys: "ctrl+enter", Variant: kit.ButtonPrimary},
			{Label: "Cancel", Keys: "esc", Variant: kit.ButtonGhost},
		},
		Hints:  []kit.Hint{{Keys: "tab", Label: "next field"}},
		Screen: s.overview(), ScreenWidth: s.w, ScreenHeight: s.h, Top: -1,
	}
	w := kit.ModalInnerWidth(mo)
	var lines []string
	add := func(block string) { lines = append(lines, strings.Split(block, "\n")...) }
	add(kit.Field(t, kit.FieldOptions{
		Width: w, Label: "Task", Help: "from a spec task: " + kit.FormatKeys(t.Glyphs, "ctrl+t"), Rows: 3, Focused: true, Cursor: true,
		Value: "Make the token cache per-user and update the callers in api/ and worker/. Keep the public Session API unchanged.",
	}))
	lines = append(lines, "")
	lines = append(lines, kit.Fit(paint(t.Text.Muted, false, "Role"), w))
	lines = append(lines, kit.Choice(t, kit.ChoiceOptions{Width: w, Options: []string{"implementer", "planner", "reviewer", "resolver"}, Selected: 0}))
	lines = append(lines, kit.Fit(paint(t.Text.Faint, false, " Writes code against the task and runs the tests before it finishes."), w))
	lines = append(lines, "")
	half := (w - 2) / 2
	base := kit.Field(t, kit.FieldOptions{Width: half, Label: "Base branch", Value: "main  " + t.Glyphs.Expanded})
	wt := kit.Fit(paint(t.Text.Muted, false, "Branch and worktree"), w-half-2) + "\n" +
		kit.Fit(paint(t.Text.Secondary, false, t.Glyphs.Branch+" mpx/token-cache-per-user"), w-half-2)
	add(kit.JoinColumns(base, "  \n  ", wt))
	lines = append(lines, "")
	lines = append(lines, kit.Fit(paint(t.Text.Faint, false, "Setup runs in the background: .env.example, pnpm install, ports 4100–4109."), w))
	mo.Body = strings.Join(lines, "\n")
	return kit.Modal(t, mo)
}
