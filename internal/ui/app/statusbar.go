package app

import "github.com/alphatechini/multi-plex/internal/ui/kit"

// blinkOn reports whether the approvals dot is lit this frame. It blinks only
// when colour animation is on; otherwise it is always lit.
func (m *Model) blinkOn() bool {
	return !m.th.Animated() || (m.frame/6)%2 == 0
}

// approvalPill is the amber pill text for the pending approvals ("" if none).
func (m *Model) approvalPill() string {
	n := len(m.approvals)
	if n == 0 {
		return ""
	}
	dot := m.th.Glyphs.Blocked
	if !m.blinkOn() {
		dot = " "
	}
	text := dot + " " + itoa(n) + " approval"
	if n != 1 {
		text += "s"
	}
	if n == 1 {
		if s := m.sessionByID(m.approvals[0].req.SessionID); s != nil {
			text += " · " + s.Slug
		}
	}
	return text
}

// renderStatus draws the status bar.
func (m *Model) renderStatus() string {
	t := m.th
	name := m.ws.Name
	if name == "" {
		name = "multiplex"
	}
	left := []kit.Segment{{Text: name, Tone: kit.ToneStrong, Bold: true}}
	var right []kit.Segment
	s := m.session()
	switch {
	case s != nil:
		left = append(left,
			kit.Segment{Text: t.Glyphs.ChevronRight + " " + s.Slug, Tone: kit.ToneDefault},
			kit.Segment{Text: kit.StateBadge(t, kitState(s.State), m.frame)},
			kit.Segment{Text: t.Glyphs.Branch + " " + s.Branch, Tone: kit.ToneMuted})
	case m.loaded && len(m.sessions) == 0:
		left = append(left,
			kit.Segment{Text: t.Glyphs.Branch + " main", Tone: kit.ToneMuted},
			kit.Segment{Text: "no sessions", Tone: kit.ToneFaint})
	}
	if pill := m.approvalPill(); pill != "" {
		right = append(right, kit.Segment{Text: pill, Tone: kit.ToneWarning, Pill: true})
	}
	if m.prefixOn {
		right = append(right, kit.Segment{Text: "PREFIX", Tone: kit.ToneAccent, Pill: true})
	}
	return kit.StatusBar(t, kit.StatusBarOptions{Width: m.w, Left: left, Right: right, Hints: m.statusHints()})
}

// statusHints are the key hints at the right end of the status bar: the
// prefix cheat sheet while the prefix is active, else the focused view's own
// hints followed by the global ones.
func (m *Model) statusHints() []kit.Hint {
	if m.prefixOn {
		return []kit.Hint{
			{Keys: "p", Label: "palette"}, {Keys: "n", Label: "new"}, {Keys: "1-5", Label: "mode"},
			{Keys: "h l", Label: "focus"}, {Keys: "a", Label: "agent"}, {Keys: "z", Label: "zoom"},
			{Keys: "?", Label: "keys"}, {Keys: "q", Label: "quit"}, {Keys: "esc", Label: "cancel"},
		}
	}
	var hints []kit.Hint
	if m.focus == regionMain && m.loaded && !m.welcome() {
		hints = append(hints, firstN(m.active().Hints(), 4)...)
	}
	return append(hints,
		kit.Hint{Keys: m.prefix + " p", Label: "palette"},
		kit.Hint{Keys: m.prefix + " n", Label: "new"},
		kit.Hint{Keys: "?", Label: "keys"})
}

func firstN[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}
