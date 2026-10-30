package codeview

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// renderPicker draws the go-to-file box over screen, in the style of the
// command palette: input row, rule, results, rule, key hints.
func (m *model) renderPicker(screen string, w, h int) string {
	th := m.th
	p := m.picker
	mo := kit.ModalOptions{Width: 72, Flush: true, Screen: screen, ScreenWidth: w, ScreenHeight: h, Top: 1}
	iw := kit.ModalInnerWidth(mo)
	if iw < 12 {
		return screen
	}
	p.in.SetWidth(max(iw-24, 8))
	var lines []string
	input := " " + paint(th.Accent.Base, true, th.Glyphs.ChevronRight) + " " + p.in.View()
	right := kit.KeyHint(th, kit.Hint{Keys: "esc", Label: "close"}) + " "
	lines = append(lines, "", spreadLine(input, right, iw), "", kit.Divider(th, kit.DividerOptions{Width: iw}))

	if len(p.items) == 0 {
		lines = append(lines, kit.Fit("  "+paint(th.Text.Muted, false, "This session has no changed files yet. Type a path to open it."), iw))
	}
	end := min(p.scroll+pickerRows, len(p.items))
	for i := p.scroll; i < end; i++ {
		it := p.items[i]
		lines = append(lines, kit.Row(th, kit.RowOptions{
			Width: iw, Selected: i == p.sel, Focused: true, Bg: th.Surface.Overlay,
			Left: " " + m.pickLeft(it, i == p.sel), Right: m.pickRight(it) + " ",
		}))
	}
	if len(p.items) > pickerRows {
		lines = append(lines, kit.Fit("   "+paint(th.Text.Faint, false, itoa(p.sel+1)+" of "+itoa(len(p.items))), iw))
	}
	lines = append(lines, "", kit.Divider(th, kit.DividerOptions{Width: iw}),
		kit.Fit(" "+kit.KeyHints(th, []kit.Hint{{Keys: "up down", Label: "move"}, {Keys: "enter", Label: "open"}}, 0), iw))
	mo.Body = strings.Join(lines, "\n")
	return kit.Modal(th, mo)
}

// spreadLine puts right at the end of a w-cell line after left.
func spreadLine(left, right string, w int) string {
	left = kit.Truncate(left, max(w-kit.Width(right)-1, 0), "…")
	return kit.PadRight(left, w-kit.Width(right)) + right
}

// pickLeft draws a result: status letter, directory, file name with the
// matched characters in the accent colour. A typed path reads "Open <path>".
func (m *model) pickLeft(it pickItem, selected bool) string {
	th := m.th
	nameC := th.Text.Primary
	if selected {
		nameC = th.Text.Strong
	}
	if it.change == nil {
		return paint(th.Text.Muted, false, "open ") + paint(nameC, selected, clean(it.path, true))
	}
	status := paint(statusColorOf(m, it.change.Status), true, statusLetter(it.change.Status))
	pos := map[int]bool{}
	for _, p := range it.pos {
		pos[p] = true
	}
	var b strings.Builder
	dirLen := len([]rune(dirOf(it.path)))
	for i, r := range []rune(it.path) {
		switch {
		case pos[i]:
			b.WriteString(paint(th.Accent.Base, true, string(r)))
		case i < dirLen:
			b.WriteString(paint(th.Text.Muted, false, string(r)))
		default:
			b.WriteString(paint(nameC, selected, string(r)))
		}
	}
	return status + "  " + b.String()
}

// pickRight draws a result's +added −removed counts.
func (m *model) pickRight(it pickItem) string {
	th := m.th
	if it.change == nil {
		return ""
	}
	var s string
	if it.change.Added > 0 {
		s += paint(th.Diff.AddSign, false, "+"+itoa(it.change.Added))
	}
	if it.change.Deleted > 0 {
		s += " " + paint(th.Diff.DelSign, false, th.Glyphs.Minus+itoa(it.change.Deleted))
	}
	return strings.TrimSpace(s)
}

// statusLetter is the one-letter change status.
func statusLetter(s vcs.FileStatus) string {
	switch s {
	case vcs.Added:
		return "A"
	case vcs.Deleted:
		return "D"
	case vcs.Renamed:
		return "R"
	default:
		return "M"
	}
}
