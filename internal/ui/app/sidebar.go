package app

import (
	"strings"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// sbKind says what a selectable sidebar entry is.
type sbKind int

// Sidebar entry kinds.
const (
	sbSession sbKind = iota
	sbSpec
	sbSteering
)

// sbItem is one selectable sidebar entry.
type sbItem struct {
	kind  sbKind
	id    core.SessionID // sbSession
	spec  string         // sbSpec: slug
	steer steeringItem   // sbSteering
}

// sidebarState is the sidebar's selectable entries, cursor and scroll.
type sidebarState struct {
	items  []sbItem
	cursor int
	scroll int
}

// sbLine is one rendered sidebar line and the entry it belongs to (-1: none).
type sbLine struct {
	text string
	item int
}

// sbRebuild recomputes the selectable entries after data changed, keeping the
// cursor on the same entry when it still exists.
func (m *Model) sbRebuild() {
	var keep sbItem
	if m.sb.cursor >= 0 && m.sb.cursor < len(m.sb.items) {
		keep = m.sb.items[m.sb.cursor]
	}
	items := make([]sbItem, 0, len(m.sessions)+len(m.specs)+len(m.info.Steering))
	for _, s := range m.orderedSessions() {
		items = append(items, sbItem{kind: sbSession, id: s.ID})
	}
	for _, sp := range m.specs {
		items = append(items, sbItem{kind: sbSpec, spec: sp.Slug})
	}
	for _, st := range m.info.Steering {
		items = append(items, sbItem{kind: sbSteering, steer: st})
	}
	m.sb.items = items
	m.sb.cursor = 0
	for i, it := range items {
		if it.kind == keep.kind && it.id == keep.id && it.spec == keep.spec && it.steer.Path == keep.steer.Path {
			m.sb.cursor = i
			return
		}
	}
	if s := m.session(); s != nil {
		m.sb.cursor = m.sbIndexOfSession(s.ID)
	}
}

func (m *Model) sbIndexOfSession(id core.SessionID) int {
	for i, it := range m.sb.items {
		if it.kind == sbSession && it.id == id {
			return i
		}
	}
	return 0
}

// sbClamp keeps the cursor in range.
func (m *Model) sbClamp() {
	m.sb.cursor = min(max(m.sb.cursor, 0), max(len(m.sb.items)-1, 0))
}

// orderedSessions returns the sessions grouped by workspace, groups in order
// of first appearance. With one workspace this is the daemon's order.
func (m *Model) orderedSessions() []core.Session {
	groups := m.workspaceGroups()
	if len(groups) <= 1 {
		return m.sessions
	}
	out := make([]core.Session, 0, len(m.sessions))
	for _, g := range groups {
		for _, s := range m.sessions {
			if s.WorkspaceID == g {
				out = append(out, s)
			}
		}
	}
	return out
}

// workspaceGroups lists the distinct workspace IDs of the sessions.
func (m *Model) workspaceGroups() []core.WorkspaceID {
	var out []core.WorkspaceID
	seen := map[core.WorkspaceID]bool{}
	for _, s := range m.sessions {
		if !seen[s.WorkspaceID] {
			seen[s.WorkspaceID] = true
			out = append(out, s.WorkspaceID)
		}
	}
	return out
}

// workspaceLabel names a workspace in the sidebar's group headers.
func (m *Model) workspaceLabel(id core.WorkspaceID) string {
	if id == m.ws.ID && m.ws.Name != "" {
		return m.ws.Name
	}
	s := string(id)
	s = strings.TrimPrefix(s, "ws_")
	return "workspace " + s[:min(len(s), 6)]
}

// highlighted is the entry drawn as selected: the cursor while the sidebar is
// focused, otherwise the current session.
func (m *Model) highlighted() int {
	if m.focus == regionSidebar {
		return m.sb.cursor
	}
	if s := m.session(); s != nil {
		return m.sbIndexOfSession(s.ID)
	}
	return -1
}

// sbLines renders the sidebar's body at inner width iw as lines tagged with
// their entry. Rail mode (iw below 8) shows one icon cell per session.
func (m *Model) sbLines(iw int) []sbLine {
	t := m.th
	sel, focused := m.highlighted(), m.focus == regionSidebar
	var out []sbLine
	add := func(item int, block string) {
		for _, l := range kit.Lines(block) {
			out = append(out, sbLine{text: l, item: item})
		}
	}
	gap := func() { out = append(out, sbLine{item: -1}) }
	now := m.now()

	gap()
	rail := iw < 8
	var prevWS core.WorkspaceID
	first := true
	groups := m.workspaceGroups()
	switch {
	case len(m.sessions) == 0 && !rail:
		add(-1, kit.Section(t, kit.SectionOptions{Width: iw, Label: "Sessions"}))
		add(-1, kit.Fit("   "+paint(t.Text.Faint, false, "none yet"), iw))
	case len(m.sessions) > 0:
		if !rail {
			add(-1, kit.Section(t, kit.SectionOptions{Width: iw, Label: "Sessions", Meta: itoa(len(m.sessions))}))
		}
		for _, s := range m.orderedSessions() {
			if len(groups) > 1 && !rail && (first || prevWS != s.WorkspaceID) {
				add(-1, kit.Fit("  "+paint(t.Text.Muted, true, m.workspaceLabel(s.WorkspaceID)), iw))
			}
			item := m.sbIndexOfSession(s.ID)
			badge := ""
			if n := m.notes[s.ID]; n > 0 {
				badge = itoa(n)
			}
			tone := kit.ToneDefault
			if s.State == core.StateBlocked {
				tone = kit.ToneWarning
			}
			add(item, kit.SessionItem(t, kit.SessionItemOptions{
				Width: iw, Name: s.Slug, State: kitState(s.State), Frame: m.frame,
				Detail: sessionDetail(s, now), Badge: badge, BadgeTone: tone,
				Selected: item == sel, Focused: focused, Unseen: s.Unseen,
			}))
			if rail {
				gap()
			}
			prevWS, first = s.WorkspaceID, false
		}
	}
	if rail {
		return out
	}

	gap()
	if len(m.specs) > 0 {
		add(-1, kit.Section(t, kit.SectionOptions{Width: iw, Label: "Specs", Meta: itoa(len(m.specs))}))
		for _, sp := range m.specs {
			item := m.sbIndexOfSpec(sp.Slug)
			add(item, kit.Row(t, kit.RowOptions{
				Width: iw, Left: "  " + paint(t.Text.Primary, false, sp.Slug),
				Right:    kit.Progress(t, kit.ProgressOptions{Width: 8, Done: sp.TasksDone, Total: sp.TasksTotal, Label: true}),
				Selected: item == sel, Focused: focused,
			}))
		}
		gap()
	}
	if len(m.info.Steering) > 0 {
		add(-1, kit.Section(t, kit.SectionOptions{Width: iw, Label: "Steering"}))
		for _, st := range m.info.Steering {
			item := m.sbIndexOfSteer(st.Path)
			add(item, kit.Row(t, kit.RowOptions{
				Width: iw, Left: "  " + paint(t.Text.Secondary, false, st.Label),
				Selected: item == sel, Focused: focused,
			}))
		}
		gap()
	}
	add(-1, kit.Section(t, kit.SectionOptions{Width: iw, Label: "Roles", Meta: itoa(m.info.Roles)}))
	add(-1, kit.Section(t, kit.SectionOptions{Width: iw, Label: "Skills", Meta: itoa(m.info.Skills)}))
	add(-1, kit.Section(t, kit.SectionOptions{Width: iw, Label: "MCP", Meta: itoa(m.info.MCP)}))
	return out
}

func (m *Model) sbIndexOfSpec(slug string) int {
	for i, it := range m.sb.items {
		if it.kind == sbSpec && it.spec == slug {
			return i
		}
	}
	return -1
}

func (m *Model) sbIndexOfSteer(path string) int {
	for i, it := range m.sb.items {
		if it.kind == sbSteering && it.steer.Path == path {
			return i
		}
	}
	return -1
}

// sbEnsureVisible scrolls the sidebar so the cursor's lines are on screen.
func (m *Model) sbEnsureVisible() {
	iw := max(m.lay.sidebarW-2, 0)
	ih := max(m.lay.bodyH-2, 0)
	if iw == 0 || ih == 0 {
		return
	}
	lines := m.sbLines(iw)
	first, last := -1, -1
	for i, l := range lines {
		if l.item == m.sb.cursor {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	m.sb.scroll = min(max(m.sb.scroll, 0), max(len(lines)-ih, 0))
	if first < 0 {
		return
	}
	if last >= m.sb.scroll+ih {
		m.sb.scroll = last - ih + 1
	}
	if first < m.sb.scroll {
		m.sb.scroll = first
	}
}

// renderSidebar draws the sidebar panel.
func (m *Model) renderSidebar() string {
	l := m.lay
	t := m.th
	po := kit.PanelOptions{Width: l.sidebarW, Height: l.bodyH, Focused: m.focus == regionSidebar, Flush: true}
	if !l.sidebarRail {
		po.Title = m.ws.Name
		if po.Title == "" {
			po.Title = "multiplex"
		}
		if s := m.session(); s != nil && s.BaseBranch != "" {
			po.Meta = t.Glyphs.Branch + " " + s.BaseBranch
		} else if m.ws.ID != "" {
			po.Meta = t.Glyphs.Branch + " main"
		}
	}
	iw, ih := kit.PanelInner(po)
	lines := m.sbLines(iw)
	scroll := min(max(m.sb.scroll, 0), max(len(lines)-ih, 0))
	var sb strings.Builder
	for i := scroll; i < min(len(lines), scroll+ih); i++ {
		if i > scroll {
			sb.WriteByte('\n')
		}
		sb.WriteString(lines[i].text)
	}
	po.Content = sb.String()
	return kit.Panel(t, po)
}
