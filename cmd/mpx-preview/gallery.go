package main

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// gallery shows the kit primitives on their own: panel frames (focused and
// unfocused), state badges, tabs, buttons, key hints, toasts, status bars.
func (s scene) gallery() string {
	t := s.t
	w := s.w
	var lines []string
	add := func(block string) { lines = append(lines, strings.Split(block, "\n")...) }
	pad := func(block string) string {
		ls := strings.Split(block, "\n")
		for i := range ls {
			ls[i] = "  " + ls[i]
		}
		return strings.Join(ls, "\n")
	}
	head := func(label string) {
		lines = append(lines, "", kit.Section(t, kit.SectionOptions{Width: w, Label: label}))
	}
	lines = append(lines, "")
	add(kit.Fit("  "+paint(t.Text.Strong, true, "Multiplex kit")+paint(t.Text.Muted, false, "  ·  theme "+t.Name), w))

	head("Agent panel frame · focused / unfocused")
	pw := min(60, (w-6)/2)
	mk := func(focused bool) string {
		po := kit.PanelOptions{Width: pw, Height: 7, Title: "auth-refactor", Icon: kit.StateDot(t, kit.StateWorking, s.frame),
			Meta: "claude", Focused: focused, Flush: true,
			Footer: kit.KeyHints(t, []kit.Hint{{Keys: "esc", Label: "interrupt"}}, 0)}
		iw, ih := kit.PanelInner(po)
		po.Content = s.agentTranscript(iw, ih+6)
		return kit.Panel(t, po)
	}
	add(pad(kit.JoinColumns(mk(true), "  ", mk(false))))

	head("States")
	var states []string
	for _, st := range []kit.State{kit.StateStarting, kit.StateWorking, kit.StateBlocked, kit.StateDone, kit.StateIdle, kit.StateError} {
		states = append(states, kit.StateBadge(t, st, s.frame))
	}
	add(kit.Fit("  "+strings.Join(states, "     "), w))

	head("Tabs · buttons · keys")
	add(kit.Fit("  "+kit.TabBar(t, kit.TabBarOptions{Tabs: modeTabs, Active: 1})+"      "+
		kit.ButtonRow(t, []kit.ButtonOptions{{Label: "Approve once", Keys: "enter", Variant: kit.ButtonPrimary}, {Label: "Deny", Keys: "esc"}, {Label: "Cancel", Keys: "esc", Variant: kit.ButtonGhost}}, 0), w))
	add(kit.Fit("  "+kit.KeyHints(t, []kit.Hint{{Keys: "ctrl+space p", Label: "palette"}, {Keys: "shift+tab", Label: "back"}, {Keys: "enter", Label: "open"}, {Keys: "up down", Label: "move"}}, 0)+"      "+
		kit.Keycap(t, "n")+" "+kit.Keycap(t, "ctrl+space p")+"      "+
		kit.Progress(t, kit.ProgressOptions{Width: 24, Done: 3, Total: 7, Label: true}), w))

	head("Toasts")
	tw := min(46, (w-10)/3)
	toasts := []string{
		kit.Toast(t, kit.ToastOptions{Width: tw, Tone: kit.ToneSuccess, Title: "3 comments sent", Body: "to auth-refactor · src/auth/session.go"}),
		kit.Toast(t, kit.ToastOptions{Width: tw, Tone: kit.ToneWarning, Title: "api-pagination needs approval", Hints: []kit.Hint{{Keys: "ctrl+space !", Label: "go there"}}}),
		kit.Toast(t, kit.ToastOptions{Width: tw, Tone: kit.ToneDanger, Title: "docs-sweep exited (code 1)", Body: "agent CLI crashed · restart with r"}),
	}
	add(pad(kit.JoinColumns(toasts[0], "  ", toasts[1], "  ", toasts[2])))

	head("Status bar · normal / attention / narrow")
	sb := func(width int, approvals bool) string {
		o := kit.StatusBarOptions{Width: width,
			Left: []kit.Segment{{Text: "multi-plex", Tone: kit.ToneStrong, Bold: true},
				{Text: t.Glyphs.ChevronRight + " auth-refactor"},
				{Text: kit.StateBadge(t, kit.StateWorking, s.frame)},
				{Text: t.Glyphs.Branch + " mpx/auth-refactor", Tone: kit.ToneMuted}},
			Hints: []kit.Hint{{Keys: "ctrl+space p", Label: "palette"}, {Keys: "ctrl+space n", Label: "new"}, {Keys: "?", Label: "keys"}}}
		if approvals {
			o.Right = []kit.Segment{{Text: t.Glyphs.Blocked + " 2 approvals", Tone: kit.ToneWarning, Pill: true}}
		}
		return kit.StatusBar(t, o)
	}
	add(pad(sb(w-4, false)))
	lines = append(lines, "")
	add(pad(sb(w-4, true)))
	lines = append(lines, "")
	add(pad(sb(min(80, w-4), true)))
	return kit.Backfill(kit.FitBlock(strings.Join(lines, "\n"), w, s.h), t.Surface.Base)
}
