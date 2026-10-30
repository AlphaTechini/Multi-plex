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
)

// newRoles are the roles offered in the dialog, with their one-line
// descriptions (docs/10).
var newRoles = []struct{ name, desc string }{
	{"orchestrator", "Breaks a goal into tasks and starts sessions for them."},
	{"planner", "Reads the code and writes a plan or a spec. Does not edit code."},
	{"implementer", "Writes code against the task and runs the tests before it finishes."},
	{"researcher", "Investigates a question and reports what it found. Read-only."},
	{"reviewer", "Reviews a branch and leaves comments. Read-only."},
	{"resolver", "Resolves merge conflicts between sessions."},
}

const defaultRole = 2 // implementer

// Fields of the dialog.
const (
	nsTask = iota
	nsRole
	nsBase
	nsFields
)

// newSessionOverlay is the new-session dialog (docs/05): task, role, base
// branch.
type newSessionOverlay struct {
	task  textarea.Model
	role  int
	base  lineEdit
	field int
	err   string
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
	return &newSessionOverlay{task: ta, role: defaultRole, base: newLineEdit(base)}, nil
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
	case nsRole:
		switch k.String() {
		case "left", "h":
			n.role = (n.role + len(newRoles) - 1) % len(newRoles)
		case "right", "l":
			n.role = (n.role + 1) % len(newRoles)
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
	cols, rows := m.lay.agentInner()
	p := protocol.SessionCreateParams{
		WorkspaceID: m.ws.ID, Task: task, Role: newRoles[n.role].name,
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

	add(fieldBox(t, w, "Task", "from a spec task: "+kit.FormatKeys(t.Glyphs, "ctrl+t"),
		strings.Split(n.task.View(), "\n"), 3, n.field == nsTask))
	lines = append(lines, "")
	roleLabel := paint(t.Text.Muted, false, "Role")
	if n.field == nsRole {
		roleLabel = paint(t.Text.Strong, true, "Role")
	}
	lines = append(lines, kit.Fit(roleLabel, w))
	names := make([]string, len(newRoles))
	for i, r := range newRoles {
		names[i] = r.name
	}
	lines = append(lines, kit.Choice(t, kit.ChoiceOptions{Width: w, Options: names, Selected: n.role, Focused: n.field == nsRole}))
	lines = append(lines, kit.Fit(paint(t.Text.Faint, false, " "+newRoles[n.role].desc), w))
	lines = append(lines, "")

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
	lines = append(lines, "")
	if n.err != "" {
		lines = append(lines, kit.Fit(paint(t.Severity.Warning, false, t.Glyphs.Blocked+" "+n.err), w))
	} else {
		lines = append(lines, kit.Fit(paint(t.Text.Faint, false, "A worktree and branch are created first; setup then runs in the background."), w))
	}
	mo.Body = strings.Join(lines, "\n")
	return kit.Modal(t, mo)
}

func orStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
