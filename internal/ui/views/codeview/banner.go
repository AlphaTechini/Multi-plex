package codeview

import (
	"image/color"
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// banner is the path header on the raised surface (docs/16 file banner): the
// directory in secondary text, the file name strong and bold, its place among
// the changed files, and on the right its change status and the cursor line.
func (m *model) banner(w int) string {
	th := m.th
	dir, file := dirOf(clean(m.path, true)), baseOf(clean(m.path, true))
	pos := ""
	if i := m.changeIndex(m.path); i >= 0 {
		pos = paint(th.Text.Muted, false, "  "+th.Glyphs.Dot+"  "+itoa(i+1)+" of "+itoa(len(m.changes)))
	}
	var meta []string
	if i := m.changeIndex(m.path); i >= 0 {
		c := m.changes[i]
		meta = append(meta, paint(statusColorOf(m, c.Status), false, string(c.Status)))
		if c.Added > 0 || c.Deleted > 0 {
			meta = append(meta, paint(th.Diff.AddSign, false, "+"+itoa(c.Added))+" "+paint(th.Diff.DelSign, false, th.Glyphs.Minus+itoa(c.Deleted)))
		}
	} else if m.src != nil {
		if lang := m.src.Language(); lang != "" {
			meta = append(meta, paint(th.Text.Muted, false, lang))
		}
	}
	switch {
	case m.src != nil:
		meta = append(meta, paint(th.Text.Muted, false, "Ln "+itoa(m.cursor+1)+" of "+itoa(m.lines())))
	case m.loading != "":
		meta = append(meta, paint(th.Text.Muted, false, "opening…"))
	}
	right := strings.Join(meta, "   ")
	name := paint(th.Text.Strong, true, file)
	room := w - 3 - kit.Width(right) - 1 - kit.Width(name) - kit.Width(pos)
	dirS := ""
	if dir != "" {
		dirS = paint(th.Text.Secondary, false, kit.TruncateLeft(dir, max(room, 0), th.Glyphs.Ellipsis))
	}
	left := paint(th.Text.Faint, false, th.Glyphs.Bar) + " " + dirS + name + pos
	gap := max(w-1-kit.Width(left)-kit.Width(right), 1)
	return kit.Backfill(kit.Fit(left+strings.Repeat(" ", gap)+right+" ", w), th.Surface.Raised)
}

// statusColorOf maps a change status to its colour.
func statusColorOf(m *model, s vcs.FileStatus) color.Color {
	th := m.th
	switch s {
	case vcs.Added:
		return th.Diff.AddSign
	case vcs.Deleted:
		return th.Diff.DelSign
	case vcs.Renamed:
		return th.Diff.Renamed
	default:
		return th.Diff.Modified
	}
}
