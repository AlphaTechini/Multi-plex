package main

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// focus names the focused region.
type focus int

const (
	focusSidebar focus = iota
	focusMain
	focusAgent
)

// session is fake sidebar data.
type session struct {
	name, detail, badge string
	state               kit.State
	badgeTone           kit.Tone
	unseen              bool
}

var sessions = []session{
	{name: "auth-refactor", detail: "implementer · 14m", badge: "3", state: kit.StateWorking},
	{name: "api-pagination", detail: "needs approval", badge: "1", badgeTone: kit.ToneWarning, state: kit.StateBlocked},
	{name: "docs-sweep", detail: "done · 6 files", state: kit.StateDone, unseen: true},
}

// screenOpts selects what the chrome shows.
type screenOpts struct {
	mode     int // active main tab
	focus    focus
	diffMode bool
	empty    bool
}

var modeTabs = []kit.Tab{
	{Label: "Overview", Badge: "3"},
	{Label: "Diff", Badge: "7"},
	{Label: "Code"},
	{Label: "Specs"},
	{Label: "Term"},
}

// screen assembles sidebar | main | agent and the status bar. mainContent
// receives the main panel's inner size.
func (s scene) screen(o screenOpts, mainContent func(w, h int) string, mainMeta string) string {
	f := computeLayout(s.w, s.h, o.diffMode)
	side := s.sidebar(f, o)
	mainP := s.mainPanel(f, o, mainContent, mainMeta)
	agent := s.agentPanel(f, o)
	body := kit.JoinColumns(side, mainP, agent)
	return body + "\n" + s.statusBar(o)
}

func (s scene) mainPanel(f frame, o screenOpts, content func(w, h int) string, meta string) string {
	tabs := modeTabs
	if o.empty {
		tabs = []kit.Tab{{Label: "Welcome"}}
	}
	po := kit.PanelOptions{
		Width: f.main, Height: f.bodyH,
		Header:  kit.TabBar(s.t, kit.TabBarOptions{Tabs: tabs, Active: o.mode}),
		Meta:    meta,
		Focused: o.focus == focusMain,
		Flush:   true,
	}
	iw, ih := kit.PanelInner(po)
	po.Content = content(iw, ih)
	return kit.Panel(s.t, po)
}

func (s scene) sidebar(f frame, o screenOpts) string {
	po := kit.PanelOptions{
		Width: f.sidebar, Height: f.bodyH,
		Focused: o.focus == focusSidebar,
		Flush:   true,
	}
	if !f.sidebarRail {
		po.Title = "multi-plex"
		po.Meta = s.t.Glyphs.Branch + " main"
	}
	iw, _ := kit.PanelInner(po)
	var lines []string
	add := func(block string) { lines = append(lines, strings.Split(block, "\n")...) }

	if f.sidebarRail {
		lines = append(lines, "")
		if !o.empty {
			for i, ss := range sessions {
				add(kit.SessionItem(s.t, kit.SessionItemOptions{
					Width: iw, Name: ss.name, State: ss.state, Frame: s.frame,
					Badge: ss.badge, BadgeTone: ss.badgeTone,
					Selected: i == 0, Focused: o.focus == focusSidebar,
				}))
				lines = append(lines, "")
			}
		}
		po.Content = strings.Join(lines, "\n")
		return kit.Panel(s.t, po)
	}

	lines = append(lines, "")
	if o.empty {
		add(kit.Section(s.t, kit.SectionOptions{Width: iw, Label: "Sessions"}))
		lines = append(lines, kit.Fit("   "+s.ink(kit.ToneFaint, "none yet"), iw))
	} else {
		add(kit.Section(s.t, kit.SectionOptions{Width: iw, Label: "Sessions", Meta: "3"}))
		for i, ss := range sessions {
			add(kit.SessionItem(s.t, kit.SessionItemOptions{
				Width: iw, Name: ss.name, State: ss.state, Frame: s.frame,
				Detail: ss.detail, Badge: ss.badge, BadgeTone: ss.badgeTone,
				Selected: i == 0, Focused: o.focus == focusSidebar, Unseen: ss.unseen,
			}))
		}
	}
	lines = append(lines, "")
	add(kit.Section(s.t, kit.SectionOptions{Width: iw, Label: "Specs", Meta: "1"}))
	add(kit.Row(s.t, kit.RowOptions{Width: iw,
		Left:  "  " + s.ink(kit.ToneDefault, "token-refresh"),
		Right: kit.Progress(s.t, kit.ProgressOptions{Width: 8, Done: 3, Total: 7, Label: true}),
	}))
	lines = append(lines, "")
	add(kit.Section(s.t, kit.SectionOptions{Width: iw, Label: "Steering"}))
	for _, f := range []string{"AGENTS.md", "CLAUDE.md", "~/.claude/CLAUDE.md", ".multiplex/"} {
		add(kit.Row(s.t, kit.RowOptions{Width: iw, Left: "  " + s.ink(kit.ToneSecondary, f)}))
	}
	lines = append(lines, "")
	for _, sec := range [][2]string{{"Roles", "4"}, {"Skills", "6"}, {"MCP", "2"}} {
		add(kit.Section(s.t, kit.SectionOptions{Width: iw, Label: sec[0], Meta: sec[1]}))
	}
	po.Content = strings.Join(lines, "\n")
	return kit.Panel(s.t, po)
}

func (s scene) agentPanel(f frame, o screenOpts) string {
	ss := sessions[0]
	if f.agentRail {
		var lines []string
		if !o.empty {
			lines = []string{kit.StateDot(s.t, ss.state, s.frame)}
		}
		return kit.Panel(s.t, kit.PanelOptions{
			Width: f.agent, Height: f.bodyH, Flush: true,
			Content: strings.Join(lines, "\n"),
		})
	}
	po := kit.PanelOptions{
		Width: f.agent, Height: f.bodyH,
		Focused: o.focus == focusAgent,
		Flush:   true,
	}
	if o.empty {
		po.Title = "Agent"
		iw, ih := kit.PanelInner(po)
		po.Content = kit.EmptyState(s.t, kit.EmptyStateOptions{
			Width: iw, Height: ih,
			Body: "The agent's own terminal appears here when a session starts.",
		})
		return kit.Panel(s.t, po)
	}
	po.Title = ss.name
	po.Icon = kit.StateDot(s.t, ss.state, s.frame)
	po.Meta = "claude"
	po.Footer = kit.KeyHints(s.t, []kit.Hint{{Keys: "esc", Label: "interrupt"}, {Keys: "ctrl+space a", Label: "zoom"}}, 0)
	iw, ih := kit.PanelInner(po)
	po.Content = s.agentTranscript(iw, ih)
	return kit.Panel(s.t, po)
}

func (s scene) statusBar(o screenOpts) string {
	t := s.t
	left := []kit.Segment{{Text: "multi-plex", Tone: kit.ToneStrong, Bold: true}}
	var right []kit.Segment
	if o.empty {
		left = append(left,
			kit.Segment{Text: t.Glyphs.Branch + " main", Tone: kit.ToneMuted},
			kit.Segment{Text: "no sessions", Tone: kit.ToneFaint})
	} else {
		left = append(left,
			kit.Segment{Text: t.Glyphs.ChevronRight + " auth-refactor", Tone: kit.ToneDefault},
			kit.Segment{Text: kit.StateBadge(t, kit.StateWorking, s.frame)},
			kit.Segment{Text: t.Glyphs.Branch + " mpx/auth-refactor", Tone: kit.ToneMuted})
		right = []kit.Segment{{Text: t.Glyphs.Blocked + " 1 approval · api-pagination", Tone: kit.ToneWarning, Pill: true}}
	}
	return kit.StatusBar(t, kit.StatusBarOptions{
		Width: s.w, Left: left, Right: right,
		Hints: []kit.Hint{{Keys: "ctrl+space p", Label: "palette"}, {Keys: "ctrl+space n", Label: "new"}, {Keys: "?", Label: "keys"}},
	})
}

// ink paints plain text in a tone (preview helper).
func (s scene) ink(tone kit.Tone, text string) string {
	return paint(kit.ToneColor(s.t, tone), false, text)
}
