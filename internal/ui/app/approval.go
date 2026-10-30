package app

import (
	"context"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// pendingApproval is a gated action waiting for the user, with the method the
// daemon will verify it with (known once approval.pending arrived).
type pendingApproval struct {
	req    core.ApprovalRequest
	method string
	code   string // one-time code for the typed-code method
}

// approvalResultMsg is the outcome of RespondApproval.
type approvalResultMsg struct {
	id      string
	outcome core.ApprovalOutcome
	d       core.ApprovalDecision
	err     error
}

type approvalMode int

const (
	apprChoose  approvalMode = iota // the buttons
	apprMessage                     // typing a deny message
	apprCode                        // typing the one-time code back
	apprWaiting                     // the check is running
)

// approvalOverlay is the human-present approval prompt (docs/04, Layer 5).
type approvalOverlay struct {
	id      string
	mode    approvalMode
	input   lineEdit
	showAll bool
	err     string
}

// methodTypedCode is the verifier that needs the code typed back.
const methodTypedCode = "typed-code"

// methodReporter is implemented by backends that know which verifier the
// daemon uses (its hello result), for approvals listed without an
// approval.pending event.
type methodReporter interface{ ApprovalMethod() string }

func (m *Model) approvalByID(id string) (pendingApproval, bool) {
	for _, a := range m.approvals {
		if a.req.ID == id {
			if a.method == "" {
				if r, ok := m.be.(methodReporter); ok {
					a.method = r.ApprovalMethod()
				}
			}
			return a, true
		}
	}
	return pendingApproval{}, false
}

// handleApprovalPending queues an approval and opens its prompt.
func (m *Model) handleApprovalPending(p protocol.ApprovalPending) tea.Cmd {
	pa := pendingApproval{req: p.Request, method: p.Method, code: p.Code}
	found := false
	for i := range m.approvals {
		if m.approvals[i].req.ID == p.Request.ID {
			m.approvals[i] = pa
			found = true
		}
	}
	if !found {
		m.approvals = append(m.approvals, pa)
	}
	m.showApproval(p.Request.ID)
	return nil
}

// showApproval opens the prompt for an approval unless it is already open.
func (m *Model) showApproval(id string) {
	for _, o := range m.overlays {
		if a, ok := o.(*approvalOverlay); ok && a.id == id {
			return
		}
	}
	m.pushOverlay(&approvalOverlay{id: id})
}

// resolveApproval removes a decided approval and closes its prompt.
func (m *Model) resolveApproval(id string) {
	for i := range m.approvals {
		if m.approvals[i].req.ID == id {
			m.approvals = append(m.approvals[:i], m.approvals[i+1:]...)
			break
		}
	}
	for i := 0; i < len(m.overlays); i++ {
		if a, ok := m.overlays[i].(*approvalOverlay); ok && a.id == id {
			m.overlays = append(m.overlays[:i], m.overlays[i+1:]...)
			i--
		}
	}
}

// respondCmd sends the user's decision. The call can take as long as the OS
// check does, so it has a generous timeout.
func (m *Model) respondCmd(p protocol.ApprovalRespondParams) tea.Cmd {
	be := m.be
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		d, err := be.RespondApproval(ctx, p)
		return approvalResultMsg{id: p.RequestID, outcome: p.Outcome, d: d, err: err}
	}
}

// handleApprovalResult closes the prompt on success and shows the outcome; a
// refusal (wrong code, failed check) leaves the prompt open with the reason.
func (m *Model) handleApprovalResult(r approvalResultMsg) tea.Cmd {
	pa, _ := m.approvalByID(r.id)
	slug := ""
	if s := m.sessionByID(pa.req.SessionID); s != nil {
		slug = s.Slug
	}
	if r.err != nil {
		for _, o := range m.overlays {
			if a, ok := o.(*approvalOverlay); ok && a.id == r.id {
				a.mode, a.err = apprChoose, r.err.Error()
				if pa.method == methodTypedCode && r.outcome == core.Approved {
					a.mode = apprCode
				}
			}
		}
		return nil
	}
	m.resolveApproval(r.id)
	var cmds []tea.Cmd
	if r.d.Outcome == core.Approved {
		cmds = append(cmds, m.addToast(toastSpec{tone: kit.ToneSuccess, title: "Approved", body: firstLine(pa.req.Command, slug)}))
	} else {
		cmds = append(cmds, m.addToast(toastSpec{tone: kit.ToneInfo, title: "Denied", body: firstLine(pa.req.Command, slug)}))
	}
	// Keep going when more are waiting: the user is already in the flow.
	if len(m.approvals) > 0 {
		m.showApproval(m.approvals[0].req.ID)
	}
	return tea.Batch(cmds...)
}

func firstLine(cmd, fallback string) string {
	if cmd == "" {
		return fallback
	}
	line, _, _ := strings.Cut(cmd, "\n")
	return line
}

func (a *approvalOverlay) update(m *Model, msg tea.Msg) (overlay, tea.Cmd) {
	pa, ok := m.approvalByID(a.id)
	if !ok {
		return nil, nil
	}
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if a.mode == apprMessage || a.mode == apprCode {
			a.input.Insert(msg.Content)
		}
		return a, nil
	case tea.KeyPressMsg:
		return a.key(m, pa, msg)
	}
	return a, nil
}

func (a *approvalOverlay) key(m *Model, pa pendingApproval, k tea.KeyPressMsg) (overlay, tea.Cmd) {
	switch a.mode {
	case apprWaiting:
		return a, nil
	case apprMessage:
		switch k.String() {
		case "esc":
			a.mode, a.input = apprChoose, lineEdit{}
		case "enter":
			a.mode = apprWaiting
			return a, m.respondCmd(protocol.ApprovalRespondParams{RequestID: a.id, Outcome: core.Denied, Message: a.input.String()})
		default:
			a.input.Key(k)
		}
		return a, nil
	case apprCode:
		switch k.String() {
		case "esc":
			a.mode, a.input, a.err = apprChoose, lineEdit{}, ""
		case "enter":
			code := strings.TrimSpace(a.input.String())
			if code == "" {
				return a, nil
			}
			a.mode, a.err = apprWaiting, ""
			return a, m.respondCmd(protocol.ApprovalRespondParams{RequestID: a.id, Outcome: core.Approved, Code: code})
		default:
			a.input.Key(k)
		}
		return a, nil
	}
	switch k.String() {
	case "enter":
		a.err = ""
		if a.commandClipped(m, pa) {
			// Never approve what is not on screen (docs/04: the prompt shows
			// the exact command).
			a.err = "The full command does not fit. Enlarge the window to approve it, or deny."
			return a, nil
		}
		if pa.method == methodTypedCode {
			a.mode, a.input = apprCode, lineEdit{}
			return a, nil
		}
		a.mode = apprWaiting
		return a, m.respondCmd(protocol.ApprovalRespondParams{RequestID: a.id, Outcome: core.Approved})
	case "esc":
		a.mode = apprWaiting
		return a, m.respondCmd(protocol.ApprovalRespondParams{RequestID: a.id, Outcome: core.Denied})
	case "m":
		a.mode, a.input, a.err = apprMessage, lineEdit{}, ""
	case "l":
		a.showAll = !a.showAll
	}
	return a, nil
}

// methodLine says how the approval will be confirmed.
func methodLine(method string) (strong, rest string) {
	switch method {
	case "windows-hello":
		return "Windows Hello", " will confirm it is you. Approval covers this one command."
	case "touch-id":
		return "Touch ID", " will confirm it is you. Approval covers this one command."
	case "polkit":
		return "Your system password", " will confirm it is you. Approval covers this one command."
	case methodTypedCode:
		return "A typed code", " will confirm it is you. Approval covers this one command."
	}
	return "A human-present check", " will confirm it is you. Approval covers this one command."
}

func approvalTitle(k core.ApprovalKind) (title, summary string) {
	switch k {
	case core.ApprovalFileWrite:
		return "Approve writing outside the worktree?", "The agent wants to write files it is not allowed to change on its own."
	case core.ApprovalFileRead:
		return "Approve reading a protected file?", "The agent wants to read files its role is not allowed to read."
	case core.ApprovalPush:
		return "Approve a push?", "The agent wants to publish commits. Pushes always need you."
	}
	return "Approve a change that cannot be undone?", "The agent wants to run a command that a safety rule stopped. It waits for you."
}

func (a *approvalOverlay) view(m *Model, screen string) string {
	pa, ok := m.approvalByID(a.id)
	if !ok {
		return screen
	}
	t := m.th
	slug := ""
	if s := m.sessionByID(pa.req.SessionID); s != nil {
		slug = s.Slug
	}
	title, _ := approvalTitle(pa.req.Kind)
	mo := kit.ModalOptions{
		Width: 92, Tone: kit.ToneWarning, Title: title,
		Meta:   paint(t.State.Blocked, false, t.Glyphs.Blocked) + " " + slug,
		Screen: screen, ScreenWidth: m.w, ScreenHeight: m.h, Top: -1,
	}
	iw := kit.ModalInnerWidth(mo)
	switch a.mode {
	case apprMessage:
		mo.Hints = []kit.Hint{{Keys: "enter", Label: "send"}, {Keys: "esc", Label: "back"}}
	case apprCode:
		mo.Hints = []kit.Hint{{Keys: "enter", Label: "confirm"}, {Keys: "esc", Label: "back"}}
	case apprWaiting:
	default:
		mo.Buttons = []kit.ButtonOptions{
			{Label: "Approve once", Keys: "enter", Variant: kit.ButtonPrimary},
			{Label: "Deny", Keys: "esc"},
			{Label: "Deny with message", Keys: "m"},
		}
	}
	mo.Body = a.fittedBody(m, pa, iw)
	return kit.Modal(t, mo)
}

// Path rows in the "Affects" field: a few by default, more when listed.
const (
	pathsCollapsed = 3
	pathsExpanded  = 8
	// modalChrome is the modal's rows besides the body: borders, title,
	// button row and the blank rows between them.
	modalChrome = 8
)

// commandClipped reports whether the command must be cut to fit the screen,
// in which case the prompt refuses to approve.
func (a *approvalOverlay) commandClipped(m *Model, pa pendingApproval) bool {
	if pa.req.Command == "" {
		return false
	}
	mo := kit.ModalOptions{Width: 92, ScreenWidth: m.w, ScreenHeight: m.h}
	body := a.body(m, pa, kit.ModalInnerWidth(mo), 0, 0)
	return len(kit.Lines(body)) > m.h-2-modalChrome
}

// fittedBody renders the body with as many path rows as the screen has room
// for, then, if the screen is still too short, clips a very long command, so
// a short terminal never pushes the buttons off the screen.
func (a *approvalOverlay) fittedBody(m *Model, pa pendingApproval, w int) string {
	want := pathsCollapsed
	if a.showAll {
		want = pathsExpanded
	}
	room := m.h - 2 - modalChrome
	var body string
	for shown := min(want, len(pa.req.Paths)); ; shown-- {
		body = a.body(m, pa, w, shown, 0)
		if shown <= 0 || len(kit.Lines(body)) <= room {
			break
		}
	}
	if over := len(kit.Lines(body)) - room; over > 0 && pa.req.Command != "" {
		rows := len(kit.Wrap(pa.req.Command, max(w-12, 8)))
		// One row of the budget goes to the "+N more lines" note.
		body = a.body(m, pa, w, 0, max(rows-over-1, 2))
	}
	return body
}

// body renders the prompt's content (docs/16, "Approval prompt").
// cmdMax > 0 limits the command to that many rows.
func (a *approvalOverlay) body(m *Model, pa pendingApproval, w, shown, cmdMax int) string {
	t := m.th
	req := pa.req
	const labelW = 10
	var lines []string
	field := func(label string, vals ...string) {
		for i, v := range vals {
			l := ""
			if i == 0 {
				l = label
			}
			lines = append(lines, kit.Fit(kit.PadRight(paint(t.Text.Muted, false, l), labelW)+v, w))
		}
	}
	_, summary := approvalTitle(req.Kind)
	for _, l := range kit.Wrap(summary, w) {
		lines = append(lines, kit.Fit(paint(t.Text.Secondary, false, l), w))
	}
	lines = append(lines, "")

	if req.Command != "" {
		// The exact command is never cut: it wraps on the sunken block.
		var cmdLines []string
		wrapped := kit.Wrap(req.Command, max(w-labelW-2, 8))
		clipped := 0
		if cmdMax > 0 && len(wrapped) > cmdMax {
			clipped, wrapped = len(wrapped)-cmdMax, wrapped[:cmdMax]
		}
		for i, l := range wrapped {
			c := paint(t.Syntax.Keyword, false, l)
			if i == 0 {
				c = paint(t.Syntax.Function, false, l)
				if name, rest, ok := strings.Cut(l, " "); ok {
					c = paint(t.Syntax.Function, false, name) + paint(t.Syntax.Keyword, false, " "+rest)
				}
			}
			cmdLines = append(cmdLines, kit.Backfill(kit.Fit(" "+c, w-labelW), t.Surface.Sunken))
		}
		if clipped > 0 {
			cmdLines = append(cmdLines, paint(t.State.Error, false, "+"+itoa(clipped)+" more "+plural(clipped, "line", "lines")+" not shown. Deny, or enlarge the window."))
		}
		field("Command", cmdLines...)
		lines = append(lines, "")
	}
	if req.Cwd != "" {
		in := ""
		if s := m.sessionByID(req.SessionID); s != nil {
			in = paint(t.Text.Primary, false, s.Slug+" worktree") + "  "
		}
		field("In", in+paint(t.Text.Muted, false, kit.TruncateLeft(req.Cwd, max(w-labelW-kit.Width(in), 8), t.Glyphs.Ellipsis)))
	}
	if why := whyLines(t.Severity.Warning, t.Text.Primary, t.Text.Muted, req); len(why) > 0 {
		field("Why", why...)
	}
	if n := len(req.Paths); n > 0 {
		vals := []string{paint(t.Text.Primary, false, itoa(n)+" "+plural(n, "path", "paths"))}
		for _, p := range req.Paths[:min(shown, n)] {
			// A long path keeps its end, which names the file.
			vals = append(vals, paint(t.Text.Secondary, false, kit.TruncateLeft(p, max(w-labelW, 8), t.Glyphs.Ellipsis)))
		}
		toggle := kit.KeyHint(t, kit.Hint{Keys: "l", Label: "list all"})
		if a.showAll {
			toggle = kit.KeyHint(t, kit.Hint{Keys: "l", Label: "fewer"})
		}
		switch {
		case n > shown:
			vals = append(vals, paint(t.Text.Muted, false, "+"+itoa(n-shown)+" more")+"  "+toggle)
		case n > pathsCollapsed && a.showAll:
			vals = append(vals, toggle)
		}
		field("Affects", vals...)
	}
	lines = append(lines, "", kit.Divider(t, kit.DividerOptions{Width: w}), "")

	switch a.mode {
	case apprCode:
		lines = append(lines, kit.Fit(paint(t.Text.Secondary, false, "Type this code to confirm it is you:"), w))
		lines = append(lines, kit.Fit("  "+paintBg(t.Text.Strong, t.Surface.Sunken, true, " "+pa.code+" "), w), "")
		lines = append(lines, a.inputBox(t, w, "Code", "", ""))
	case apprMessage:
		lines = append(lines, a.inputBox(t, w, "Message to the agent", "sent with the denial", "Say what to do instead"))
	case apprWaiting:
		strong, _ := methodLine(pa.method)
		lines = append(lines, kit.Fit(paint(t.State.Blocked, false, t.Glyphs.Ellipsis)+" "+paint(t.Text.Strong, false, "Waiting for "+strong)+
			paint(t.Text.Muted, false, " — confirm on this computer"), w))
	default:
		strong, rest := methodLine(pa.method)
		for _, l := range kit.Wrap(paint(t.Text.Strong, false, strong)+paint(t.Text.Secondary, false, rest), w) {
			lines = append(lines, kit.Fit(l, w))
		}
		lines = append(lines, kit.Fit(paint(t.Text.Faint, false, "Session-wide approval for this rule is off."), w))
	}
	if a.err != "" {
		lines = append(lines, kit.Fit(paint(t.State.Error, false, t.Glyphs.Cross+" "+a.err), w))
	}
	return strings.Join(lines, "\n")
}

// whyLines turns the request's layer text ("Layer 2 writes outside…; Layer 4
// recursive delete") and rule into one line per layer, the rule on the last.
func whyLines(accent, primary, muted color.Color, req core.ApprovalRequest) []string {
	var out []string
	parts := strings.Split(req.Layer, ";")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		head, rest := p, ""
		if f := strings.Fields(p); len(f) >= 2 && strings.EqualFold(f[0], "layer") {
			head = f[0] + " " + f[1]
			rest = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p), head))
		}
		line := paint(accent, false, head)
		if rest != "" {
			line += paint(primary, false, "  "+rest)
		}
		if i == len(parts)-1 && req.Rule != "" {
			line += paint(muted, false, "  rule "+req.Rule)
		}
		out = append(out, line)
	}
	if len(out) == 0 && req.Rule != "" {
		out = append(out, paint(muted, false, "rule "+req.Rule))
	}
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// inputBox draws the prompt's one-line input with its cursor.
func (a *approvalOverlay) inputBox(t *theme.Theme, w int, label, help, placeholder string) string {
	cursor := func(s string) string { return paintBg(t.Text.Strong, t.Accent.Base, false, s) }
	plain := func(s string) string { return paint(t.Text.Primary, false, s) }
	line := a.input.Render(w-4, plain, cursor)
	if a.input.String() == "" && placeholder != "" {
		line = cursor(" ") + paint(t.Text.Faint, false, placeholder)
	}
	return fieldBox(t, w, label, help, []string{line}, 1, true)
}
