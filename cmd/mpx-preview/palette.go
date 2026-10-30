package main

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

type command struct {
	group, label, keys string
}

var commands = []command{
	{"Sessions", "New session", "ctrl+space n"},
	{"Sessions", "New session from spec task", ""},
	{"Specs", "New spec from an idea", ""},
	{"Insights", "Jump to newest card", "ctrl+space 1"},
	{"Recent", "Network: allow host for this session", ""},
}

// palette is the command palette over the Diff screen.
func (s scene) palette() string {
	t := s.t
	query := "new"
	mo := kit.ModalOptions{Width: 78, Flush: true, Screen: s.diff(), ScreenWidth: s.w, ScreenHeight: s.h, Top: max(s.h/7, 2)}
	w := kit.ModalInnerWidth(mo)
	var lines []string
	input := " " + paint(t.Accent.Base, true, t.Glyphs.ChevronRight) + " " + paint(t.Text.Strong, false, query) +
		kit.Backfill(" ", t.Accent.Base)
	lines = append(lines, "", kit.Fit(spreadPlain(input, kit.KeyHint(t, kit.Hint{Keys: "esc", Label: "close"})+" ", w), w), "")
	lines = append(lines, kit.Divider(t, kit.DividerOptions{Width: w}))
	group := ""
	sel := 0
	for i, c := range commands {
		if c.group != group {
			lines = append(lines, "", kit.Section(t, kit.SectionOptions{Width: w, Label: c.group}))
			group = c.group
		}
		label := highlightMatch(s, c.label, query, i == sel)
		right := ""
		if c.keys != "" {
			right = paint(t.Text.Muted, false, kit.FormatKeys(t.Glyphs, c.keys))
		}
		lines = append(lines, kit.Row(t, kit.RowOptions{Width: w, Left: " " + label, Right: right + " ", Selected: i == sel, Focused: true, Bg: t.Surface.Overlay}))
	}
	lines = append(lines, "", kit.Divider(t, kit.DividerOptions{Width: w}))
	lines = append(lines, kit.Fit(" "+kit.KeyHints(t, []kit.Hint{{Keys: "up down", Label: "move"}, {Keys: "enter", Label: "run"}, {Keys: "tab", Label: "complete"}, {Keys: "/", Label: "slash commands"}}, 0), w))
	mo.Body = strings.Join(lines, "\n")
	return kit.Modal(t, mo)
}

// highlightMatch paints the characters of label matching query (fuzzy, in
// order) in the accent colour.
func highlightMatch(s scene, label, query string, selected bool) string {
	t := s.t
	base := t.Text.Primary
	if selected {
		base = t.Text.Strong
	}
	q := []rune(strings.ToLower(strings.ReplaceAll(query, " ", "")))
	var b strings.Builder
	qi := 0
	for _, r := range label {
		if qi < len(q) && strings.ToLower(string(r)) == string(q[qi]) {
			b.WriteString(paint(t.Accent.Base, true, string(r)))
			qi++
			continue
		}
		b.WriteString(paint(base, false, string(r)))
	}
	return b.String()
}

func spreadPlain(left, right string, w int) string {
	return kit.PadRight(left, w-kit.Width(right)) + right
}
