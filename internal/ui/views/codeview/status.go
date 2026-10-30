package codeview

import (
	"time"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// statusRow is the row under the banner: the follow indicator while follow
// mode is on ("following · lines 38–52 edited 2s ago", or the amber
// "following paused · f to resume" after the user scrolled away), otherwise
// the keys of the view.
func (m *model) statusRow(w int) string {
	th := m.th
	var s string
	switch {
	case m.follow && m.paused:
		s = paint(th.Severity.Warning, false, th.Glyphs.Blocked) + " " + paint(th.Text.Strong, true, "following paused") +
			paint(th.Text.Faint, false, " "+th.Glyphs.Dot+" ") + kit.KeyHint(th, kit.Hint{Keys: "f", Label: "to resume"})
		if m.last != nil {
			s += paint(th.Text.Muted, false, "   newest edit "+m.editLabel())
		}
	case m.follow:
		s = paint(th.State.Working, false, th.Glyphs.Working) + " " + paint(th.Text.Strong, true, "following") +
			paint(th.Text.Faint, false, " "+th.Glyphs.Dot+" ")
		if m.last != nil {
			s += paint(th.Text.Muted, false, m.editLabel())
		} else {
			s += paint(th.Text.Muted, false, "waiting for the agent's next edit")
		}
		s += paint(th.Text.Faint, false, "   ") + kit.KeyHint(th, kit.Hint{Keys: "f", Label: "stop"})
	default:
		s = kit.KeyHints(th, []kit.Hint{
			{Keys: "f", Label: "follow"}, {Keys: "p", Label: "go to file"},
			{Keys: "[ ]", Label: "prev/next change"}, {Keys: "e", Label: "editor"},
		}, max(w-2, 0))
	}
	return kit.Backfill(kit.Fit("  "+s, w), th.Surface.Base)
}

// editLabel describes the newest edit: "src/auth/cache.go L12–L30, 2s ago".
func (m *model) editLabel() string {
	e := m.last
	if e == nil {
		return ""
	}
	lines := ""
	if e.StartLine > 0 {
		lines = " L" + itoa(e.StartLine)
		if e.EndLine > e.StartLine {
			lines += "–L" + itoa(e.EndLine)
		}
	}
	return clean(e.Path, true) + lines + ", " + ago(m.clock.Sub(e.At))
}

// ago formats a short relative time.
func ago(d time.Duration) string {
	s := int(d.Seconds())
	switch {
	case s < 5:
		return "just now"
	case s < 60:
		return itoa(s) + "s ago"
	case s < 3600:
		return itoa(s/60) + "m ago"
	default:
		return itoa(s/3600) + "h ago"
	}
}
