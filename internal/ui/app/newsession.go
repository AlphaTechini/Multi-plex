package app

import (
	"context"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/spec"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// fallbackRole is offered when the daemon's role list could not be loaded,
// so the dialog still works (the daemon applies its own default role).
var fallbackRole = protocol.RoleInfo{
	Name: "implementer", Description: "Writes code against the task and runs the tests before it finishes.", Default: true,
}

// Fields of the dialog.
const (
	nsTask = iota
	nsAgent
	nsRole
	nsBase
	nsFields
)

// newSessionOverlay is the new-session dialog (docs/05): task, agent, role,
// base branch. The agent and role lists come from the daemon.
type newSessionOverlay struct {
	task   textarea.Model
	agents []protocol.AgentInfo
	agent  int
	roles  []protocol.RoleInfo
	role   int
	base   lineEdit
	field  int
	err    string
}

// catalogMsg is a fresh agent and role list from the daemon.
type catalogMsg struct {
	agents []protocol.AgentInfo
	roles  []protocol.RoleInfo
	err    error
}

// sessionCreatedMsg is the result of CreateSession.
type sessionCreatedMsg struct {
	s   core.Session
	err error
}

// specTaskMsg carries the text of the first open spec task (ctrl+t).
type specTaskMsg struct {
	text string
	err  error
}

func newNewSession(m *Model) (*newSessionOverlay, tea.Cmd) {
	t := m.th
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Placeholder = "What should the agent do?"
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.SetWidth(74)
	st := textarea.Styles{}
	for _, s := range []*textarea.StyleState{&st.Focused, &st.Blurred} {
		s.Text = lipgloss.NewStyle().Foreground(t.Text.Primary)
		s.Placeholder = lipgloss.NewStyle().Foreground(t.Text.Faint)
		s.CursorLine = lipgloss.NewStyle()
		s.EndOfBuffer = lipgloss.NewStyle()
	}
	st.Cursor = textarea.CursorStyle{Color: t.Accent.Base, Shape: tea.CursorBlock, Blink: false}
	ta.SetStyles(st)
	ta.Focus()
	base := "main"
	if s := m.session(); s != nil && s.BaseBranch != "" {
		base = s.BaseBranch
	}
	n := &newSessionOverlay{task: ta, base: newLineEdit(base)}
	n.setCatalog(m.agents, m.roles)
	return n, m.catalogCmd()
}

// setCatalog installs the agent and role lists, keeping the current choices
// by name. The defaults are preselected the first time (and an unavailable
// default gives way to the first agent that is installed).
func (n *newSessionOverlay) setCatalog(agents []protocol.AgentInfo, roles []protocol.RoleInfo) {
	prevAgent, prevRole := n.agentName(), n.roleName()
	if len(agents) == 0 {
		agents = []protocol.AgentInfo{{DisplayName: "Default agent", Available: true}}
	}
	if len(roles) == 0 {
		roles = []protocol.RoleInfo{fallbackRole}
	}
	n.agents, n.roles = agents, roles
	n.agent = pickAgent(agents, prevAgent)
	n.role = pickRole(roles, prevRole)
}

// agentName and roleName are the chosen names ("" before a catalog is set).
func (n *newSessionOverlay) agentName() string {
	if n.agent < len(n.agents) {
		return n.agents[n.agent].Name
	}
	return ""
}

func (n *newSessionOverlay) roleName() string {
	if n.role < len(n.roles) {
		return n.roles[n.role].Name
	}
	return ""
}

// pickAgent returns the index of keep, else of the default agent when it is
// installed, else of the first installed agent, else 0.
func pickAgent(agents []protocol.AgentInfo, keep string) int {
	if keep != "" {
		for i, a := range agents {
			if a.Name == keep {
				return i
			}
		}
	}
	for i, a := range agents {
		if a.Default && a.Available {
			return i
		}
	}
	for i, a := range agents {
		if a.Available {
			return i
		}
	}
	return 0
}

// pickRole returns the index of keep, else of the default role, else 0.
func pickRole(roles []protocol.RoleInfo, keep string) int {
	for i, r := range roles {
		if keep != "" && r.Name == keep {
			return i
		}
	}
	for i, r := range roles {
		if r.Default {
			return i
		}
	}
	return 0
}

// catalogCmd fetches the agent and role lists, so the dialog shows what is
// installed and defined now.
func (m *Model) catalogCmd() tea.Cmd {
	be, ws := m.be, m.ws.ID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var out catalogMsg
		agents, aerr := be.Agents(ctx)
		roles, rerr := be.Roles(ctx, ws)
		out.agents, out.roles = agents, roles
		if aerr != nil {
			out.agents, out.err = nil, aerr
		}
		if rerr != nil {
			out.roles, out.err = nil, rerr
		}
		return out
	}
}

// handleCatalog stores a fresh catalog and offers it to the open dialog.
func (m *Model) handleCatalog(c catalogMsg) tea.Cmd {
	if c.agents != nil {
		m.agents = c.agents
	}
	if c.roles != nil {
		m.roles = c.roles
		m.info.Roles = len(c.roles)
	}
	for _, o := range m.overlays {
		if n, ok := o.(*newSessionOverlay); ok {
			n.setCatalog(m.agents, m.roles)
		}
	}
	return nil
}

// slugPreview mirrors how the daemon derives a session name from the task:
// the first three words, lower case, joined with dashes.
func slugPreview(task string) string {
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(task), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		words = append(words, w)
		if len(words) == 3 {
			break
		}
	}
	return strings.Join(words, "-")
}

func (n *newSessionOverlay) update(m *Model, msg tea.Msg) (overlay, tea.Cmd) {
	switch msg := msg.(type) {
	case specTaskMsg:
		if msg.err != nil || msg.text == "" {
			n.err = "No open spec task to start from"
			return n, nil
		}
		n.task.SetValue(msg.text)
		n.err = ""
		return n, nil
	case tea.PasteMsg:
		switch n.field {
		case nsTask:
			n.task.InsertString(msg.Content)
		case nsBase:
			n.base.Insert(msg.Content)
		}
		return n, nil
	case tea.KeyPressMsg:
		return n.key(m, msg)
	}
	return n, nil
}

// cycle moves a choice by d, wrapping.
func cycle(i, d, n int) int {
	if n == 0 {
		return 0
	}
	return (i + d + n) % n
}

func (n *newSessionOverlay) key(m *Model, k tea.KeyPressMsg) (overlay, tea.Cmd) {
	n.err = ""
	switch k.String() {
	case "esc":
		return nil, nil
	case "ctrl+enter", "ctrl+s", "alt+enter":
		return n.submit(m)
	case "tab":
		n.focusField((n.field + 1) % nsFields)
		return n, nil
	case "shift+tab":
		n.focusField((n.field + nsFields - 1) % nsFields)
		return n, nil
	case "ctrl+t":
		return n, m.specTaskCmd()
	}
	switch n.field {
	case nsTask:
		var cmd tea.Cmd
		n.task, cmd = n.task.Update(k)
		return n, cmd
	case nsAgent:
		switch k.String() {
		case "left", "h":
			n.agent = cycle(n.agent, -1, len(n.agents))
		case "right", "l":
			n.agent = cycle(n.agent, 1, len(n.agents))
		case "enter":
			return n.submit(m)
		}
	case nsRole:
		switch k.String() {
		case "left", "h":
			n.role = cycle(n.role, -1, len(n.roles))
		case "right", "l":
			n.role = cycle(n.role, 1, len(n.roles))
		case "enter":
			return n.submit(m)
		}
	case nsBase:
		if k.String() == "enter" {
			return n.submit(m)
		}
		n.base.Key(k)
	}
	return n, nil
}

func (n *newSessionOverlay) focusField(f int) {
	n.field = f
	if f == nsTask {
		n.task.Focus()
	} else {
		n.task.Blur()
	}
}

// submit validates the dialog and asks the daemon to create the session.
func (n *newSessionOverlay) submit(m *Model) (overlay, tea.Cmd) {
	task := strings.TrimSpace(n.task.Value())
	if task == "" {
		n.err = "Describe the task first"
		n.focusField(nsTask)
		return n, nil
	}
	if a := n.agents[n.agent]; !a.Available {
		n.err = a.DisplayName + " is not installed. Pick another agent or install it first."
		n.focusField(nsAgent)
		return n, nil
	}
	cols, rows := m.lay.agentInner()
	p := protocol.SessionCreateParams{
		WorkspaceID: m.ws.ID, Task: task, Agent: n.agentName(), Role: n.roleName(),
		BaseBranch: strings.TrimSpace(n.base.String()), Cols: cols, Rows: rows,
	}
	be := m.be
	cmd := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		s, err := be.CreateSession(ctx, p)
		return sessionCreatedMsg{s: s, err: err}
	}
	return nil, tea.Batch(cmd, m.addToast(toastSpec{tone: kit.ToneInfo, title: "Creating session", body: kit.Truncate(task, 36, "…")}))
}

// handleCreated selects the new session.
func (m *Model) handleCreated(r sessionCreatedMsg) tea.Cmd {
	if r.err != nil {
		return m.addToast(toastSpec{tone: kit.ToneDanger, title: "Could not create the session", body: r.err.Error()})
	}
	m.upsertSession(r.s)
	return tea.Batch(m.selectSession(r.s.ID), m.setFocus(regionMain))
}

// specTaskCmd loads the first unfinished task of the first spec that has one.
func (m *Model) specTaskCmd() tea.Cmd {
	be, ws := m.be, m.ws.ID
	var slugs []string
	for _, sp := range m.specs {
		if sp.TasksDone < sp.TasksTotal {
			slugs = append(slugs, sp.Slug)
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, slug := range slugs {
			doc, err := be.Spec(ctx, ws, slug)
			if err != nil {
				continue
			}
			for _, t := range spec.ParseTasks(doc.Tasks) {
				if !t.Done {
					return specTaskMsg{text: "Implement task " + t.ID + " of spec " + slug + ": " + t.Title}
				}
			}
		}
		return specTaskMsg{}
	}
}

func (n *newSessionOverlay) view(m *Model, screen string) string {
	t := m.th
	mo := kit.ModalOptions{
		Width: 84, Title: "New session", Meta: m.ws.Name,
		Buttons: []kit.ButtonOptions{
			{Label: "Start session", Keys: "ctrl+enter", Variant: kit.ButtonPrimary},
			{Label: "Cancel", Keys: "esc", Variant: kit.ButtonGhost},
		},
		Hints:  []kit.Hint{{Keys: "tab", Label: "next field"}},
		Screen: screen, ScreenWidth: m.w, ScreenHeight: m.h, Top: -1,
	}
	w := kit.ModalInnerWidth(mo)
	n.task.SetWidth(max(w-4, 10))
	var lines []string
	add := func(block string) { lines = append(lines, strings.Split(block, "\n")...) }
	// On a short terminal the blank rows between the sections go, so the
	// dialog (and its buttons) still fits.
	gap := func() {
		if m.h >= 30 {
			lines = append(lines, "")
		}
	}

	add(fieldBox(t, w, "Task", "from a spec task: "+kit.FormatKeys(t.Glyphs, "ctrl+t"),
		strings.Split(n.task.View(), "\n"), 3, n.field == nsTask))
	gap()
	lines = append(lines, n.choiceRows(t, w, "Agent", nsAgent, agentLabels(n.agents), n.agent, n.agentDetail(t))...)
	gap()
	lines = append(lines, n.choiceRows(t, w, "Role", nsRole, roleNames(n.roles), n.role,
		paint(t.Text.Faint, false, " "+n.roles[n.role].Description))...)
	gap()

	half := (w - 2) / 2
	cursor := func(s string) string { return paintBg(t.Text.Strong, t.Accent.Base, false, s) }
	plain := func(s string) string { return paint(t.Text.Primary, false, s) }
	baseVal := plain(n.base.String())
	if n.field == nsBase {
		baseVal = n.base.Render(half-8, plain, cursor)
	}
	base := fieldBox(t, half, "Base branch", "", []string{baseVal + paint(t.Text.Muted, false, "  "+t.Glyphs.Expanded)}, 1, n.field == nsBase)
	branch := "mpx/" + orStr(slugPreview(n.task.Value()), "…")
	wt := kit.Fit(paint(t.Text.Muted, false, "Branch and worktree"), w-half-2) + "\n" +
		kit.Fit(paint(t.Text.Secondary, false, t.Glyphs.Branch+" "+branch), w-half-2)
	add(kit.JoinColumns(base, "  \n  ", wt))
	gap()
	if n.err != "" {
		lines = append(lines, kit.Fit(paint(t.Severity.Warning, false, t.Glyphs.Blocked+" "+n.err), w))
	} else {
		for _, l := range kit.Wrap("A worktree and branch are created first; setup then runs in the background.", w) {
			lines = append(lines, kit.Fit(paint(t.Text.Faint, false, l), w))
		}
	}
	mo.Body = strings.Join(lines, "\n")
	return kit.Modal(t, mo)
}

// choiceRows draws a labelled choice with a detail line under it.
func (n *newSessionOverlay) choiceRows(t *theme.Theme, w int, label string, field int, options []string, sel int, detail string) []string {
	l := paint(t.Text.Muted, false, label)
	if n.field == field {
		l = paint(t.Text.Strong, true, label)
	}
	return []string{
		kit.Fit(l, w),
		kit.Choice(t, kit.ChoiceOptions{Width: w, Options: options, Selected: sel, Focused: n.field == field}),
		kit.Fit(detail, w),
	}
}

// agentDetail is the line under the agent choice: where the agent was found,
// or why it cannot be used.
func (n *newSessionOverlay) agentDetail(t *theme.Theme) string {
	a := n.agents[n.agent]
	if !a.Available {
		msg := a.Detail
		if msg == "" {
			msg = a.DisplayName + " was not found on PATH"
		}
		return paint(t.Severity.Warning, false, " "+t.Glyphs.Blocked+" "+msg)
	}
	if a.Detail == "" {
		return ""
	}
	return paint(t.Text.Faint, false, " "+a.Detail)
}

func agentLabels(as []protocol.AgentInfo) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = orStr(a.DisplayName, a.Name)
	}
	return out
}

func roleNames(rs []protocol.RoleInfo) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}

func orStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
