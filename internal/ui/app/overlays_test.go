package app

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend/fake"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

func calls(h *harness, name string) int {
	n := 0
	for _, c := range h.be.Calls() {
		if c == name {
			n++
		}
	}
	return n
}

func TestApprovalPendingEventOpensModal(t *testing.T) {
	h := newHarness(t, 180, 48)
	if len(h.m.overlays) != 0 {
		t.Fatal("the approval loaded at startup must not open by itself")
	}
	// A new gated action arrives: the prompt opens by itself.
	req := core.ApprovalRequest{
		ID: "apr_new", SessionID: fake.SessionAuth, Kind: core.ApprovalCommand,
		Command: "git push --force origin mpx/auth-refactor", Cwd: "/work/auth-refactor",
		Layer: "Layer 5 pushes need approval", Rule: "push · .multiplex/rules.toml:3",
	}
	h.be.Emit(protocol.TopicApprovalPending, protocol.ApprovalPending{Request: req, Method: "windows-hello"})
	h.pump()
	if len(h.m.overlays) != 1 || len(h.m.approvals) != 2 {
		t.Fatalf("overlays %d approvals %d", len(h.m.overlays), len(h.m.approvals))
	}
	out := h.plain()
	for _, want := range []string{"Approve a change that cannot be undone?", "git push --force", "Layer 5", "rule push", "Windows Hello", "Approve once", "Deny with message", "2 approvals"} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	h.shot("approval-event")
}

func TestApprovalApproveOnce(t *testing.T) {
	h := newHarness(t, 180, 48, fake.WithApprovalMethod("windows-hello"))
	h.keys("ctrl+\\", "!")
	if len(h.m.overlays) != 1 {
		t.Fatal("prefix ! must open the pending approval")
	}
	h.key("enter")
	h.settle()
	if calls(h, "RespondApproval") != 1 {
		t.Fatalf("RespondApproval called %d times", calls(h, "RespondApproval"))
	}
	if len(h.m.overlays) != 0 || len(h.m.approvals) != 0 {
		t.Fatalf("overlays %d approvals %d after approval", len(h.m.overlays), len(h.m.approvals))
	}
	if !strings.Contains(h.plain(), "Approved") {
		t.Fatal("no confirmation toast")
	}
	h.pump()
	if s := h.m.sessionByID(fake.SessionAPI); s.State != core.StateWorking {
		t.Fatalf("session state = %s", s.State)
	}
	// The pill is gone.
	if strings.Contains(h.plain(), "approval") && strings.Contains(h.plain(), "1 approval") {
		t.Fatal("pill still shown")
	}
	// Keys are not sent twice: waiting ignores input.
}

// While the OS check runs the prompt shows it and ignores keys.
func TestApprovalWaitingState(t *testing.T) {
	h := newHarness(t, 180, 48, fake.WithApprovalMethod("windows-hello"), fake.WithApprovalDelay(400*time.Millisecond))
	h.keys("ctrl+\\", "!")
	h.key("enter")
	a, ok := h.m.overlays[0].(*approvalOverlay)
	if !ok || a.mode != apprWaiting {
		t.Fatalf("overlay %T after enter", h.m.overlays[0])
	}
	if out := h.plain(); !strings.Contains(out, "Waiting for Windows Hello") && !strings.Contains(out, "Waiting for") {
		t.Fatalf("no waiting state:\n%s", out)
	}
	h.key("esc")
	h.key("m")
	if a.mode != apprWaiting || len(h.m.overlays) != 1 {
		t.Fatal("keys must be ignored while the check runs")
	}
	h.shot("approval-waiting")
	// The result arrives: the prompt closes.
	h.m.Update(approvalResultMsg{id: fake.ApprovalID, outcome: core.Approved, d: core.ApprovalDecision{RequestID: fake.ApprovalID, Outcome: core.Approved}})
	if len(h.m.overlays) != 0 {
		t.Fatal("result must close the prompt")
	}
}

func TestApprovalDenyAndMessage(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "!")
	h.key("esc") // deny
	h.settle()
	audit, _ := h.be.ApprovalAudit(t.Context(), fake.SessionAPI)
	if len(audit) != 1 || audit[0].Decision.Outcome != core.Denied {
		t.Fatalf("audit = %+v", audit)
	}
	if len(h.m.overlays) != 0 || len(h.m.approvals) != 0 {
		t.Fatal("prompt must close after deny")
	}

	// Deny with a message.
	h = newHarness(t, 180, 48)
	h.keys("ctrl+\\", "!")
	h.key("m")
	if h.m.overlays[0].(*approvalOverlay).mode != apprMessage {
		t.Fatal("m must open the message input")
	}
	h.typeText("regenerate in place")
	if !strings.Contains(h.plain(), "regenerate in place") {
		t.Fatal("typed message not shown")
	}
	h.key("esc") // back to the buttons, not a denial
	if calls(h, "RespondApproval") != 0 || h.m.overlays[0].(*approvalOverlay).mode != apprChoose {
		t.Fatal("esc in the message input must go back")
	}
	h.key("m")
	h.typeText("regenerate in place")
	h.key("enter")
	h.settle()
	audit, _ = h.be.ApprovalAudit(t.Context(), fake.SessionAPI)
	if len(audit) != 1 || audit[0].Decision.Outcome != core.Denied || audit[0].Decision.Message != "regenerate in place" {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestApprovalTypedCode(t *testing.T) {
	h := newHarness(t, 180, 48, fake.WithApprovalMethod("typed-code"))
	// The code arrives with the pending event.
	h.be.Emit(protocol.TopicApprovalPending, protocol.ApprovalPending{
		Request: h.m.approvals[0].req, Method: "typed-code", Code: "7K3-Q9",
	})
	h.pump()
	if len(h.m.overlays) != 1 {
		t.Fatal("prompt not open")
	}
	h.key("enter")
	out := h.plain()
	if !strings.Contains(out, "7K3-Q9") || !strings.Contains(out, "Code") {
		t.Fatalf("typed-code prompt must show the code and an input:\n%s", out)
	}
	if calls(h, "RespondApproval") != 0 {
		t.Fatal("enter must not approve before the code is typed")
	}
	h.typeText("WRONG")
	h.key("enter")
	h.settle()
	if len(h.m.overlays) != 1 {
		t.Fatal("a wrong code must keep the prompt open")
	}
	if a := h.m.overlays[0].(*approvalOverlay); a.err == "" || a.mode != apprCode {
		t.Fatalf("no error shown: %+v", a)
	}
	// Retype correctly.
	for range 5 {
		h.key("backspace")
	}
	h.typeText("7K3-Q9")
	h.key("enter")
	h.settle()
	if len(h.m.overlays) != 0 || len(h.m.approvals) != 0 {
		t.Fatalf("right code must approve: overlays %d approvals %d", len(h.m.overlays), len(h.m.approvals))
	}
}

func TestApprovalResolvedElsewhere(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "!")
	h.be.Emit(protocol.TopicApprovalDone, core.ApprovalDecision{RequestID: fake.ApprovalID, Outcome: core.Approved})
	h.pump()
	if len(h.m.overlays) != 0 || len(h.m.approvals) != 0 {
		t.Fatal("a decision made elsewhere must close the prompt")
	}
}

func TestApprovalListAll(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "!")
	out := h.plain()
	for _, want := range []string{"session_refresh.json", "session_expiry.json", "session_revoke.json", "+3 more", "list all"} {
		if !strings.Contains(out, want) {
			t.Fatalf("collapsed Affects lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "client_page1.json") {
		t.Fatal("only three paths show at first")
	}
	h.key("l")
	out = h.plain()
	if !strings.Contains(out, "client_last.json") || strings.Contains(out, "more") {
		t.Fatalf("l must list all six paths:\n%s", out)
	}
	assertRect(t, h.screen(), 180, 48)
}

// TestApprovalNeverOverflows opens the prompt with many long paths on
// terminals of several heights: the prompt must keep its buttons on screen.
func TestApprovalNeverOverflows(t *testing.T) {
	var paths []string
	for i := 0; i < 20; i++ {
		paths = append(paths, "internal/some/very/deep/package/with/a/long/name/"+strings.Repeat("sub/", 8)+"file_"+itoa(i)+".go")
	}
	for _, size := range [][2]int{{180, 48}, {110, 32}, {90, 24}, {80, 24}} {
		h := newHarness(t, size[0], size[1])
		req := core.ApprovalRequest{
			ID: "apr_many", SessionID: fake.SessionAPI, Kind: core.ApprovalFileWrite, Tool: "Write",
			Cwd: "/wt", Paths: paths, Layer: "Layer 2 writes outside the worktree", Rule: "outside · rules:1",
		}
		h.be.Emit(protocol.TopicApprovalPending, protocol.ApprovalPending{Request: req, Method: "windows-hello"})
		h.pump()
		for _, expand := range []bool{false, true} {
			if expand {
				h.key("l")
			}
			out := h.plain()
			if !strings.Contains(out, "Approve once") || !strings.Contains(out, "Deny") {
				t.Errorf("%dx%d expand=%v: buttons are off screen:\n%s", size[0], size[1], expand, out)
			}
			if !strings.Contains(out, "20 paths") {
				t.Errorf("%dx%d expand=%v: path count missing", size[0], size[1], expand)
			}
			assertRect(t, h.screen(), size[0], size[1])
		}
	}
}

func TestPaletteFiltering(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "p")
	p := h.m.overlays[0].(*paletteOverlay)
	all := len(p.results)
	if all < 20 {
		t.Fatalf("only %d palette entries", all)
	}
	titles := func() []string {
		var out []string
		for _, r := range p.results {
			out = append(out, r.act.title)
		}
		return out
	}
	has := func(s string) bool {
		for _, x := range titles() {
			if x == s {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"New session", "Switch to session docs-sweep", "Switch to Diff", "Toggle agent panel", "Open spec token-refresh", "Open AGENTS.md", "Command palette"} {
		if !has(want) {
			t.Errorf("palette lacks %q", want)
		}
	}
	h.typeText("swdoc")
	if got := titles(); len(got) != 1 || got[0] != "Switch to session docs-sweep" {
		t.Fatalf("swdoc -> %v", got)
	}
	// Matched characters are highlighted (accent bold).
	if !strings.Contains(h.screen(), "\x1b[1;38;2;55;148;255mS") {
		t.Error("matched characters not highlighted")
	}
	for range 5 {
		h.key("backspace")
	}
	h.typeText("zoom")
	if got := titles(); len(got) == 0 || got[0] != "Zoom focused region" {
		t.Fatalf("zoom -> %v", got)
	}
	for range 4 {
		h.key("backspace")
	}
	h.typeText("xyzq")
	if len(p.results) != 0 || !strings.Contains(h.plain(), "No matching command") {
		t.Fatal("no-match state")
	}
	assertRect(t, h.screen(), 180, 48)
	for range 4 {
		h.key("backspace")
	}
	if len(p.results) != all {
		t.Fatal("clearing the query must restore every entry")
	}
	// Navigation wraps; enter runs the selected action.
	h.key("up")
	if p.sel != len(p.results)-1 {
		t.Fatalf("up from the first must wrap, sel=%d", p.sel)
	}
	h.key("esc")
	if len(h.m.overlays) != 0 {
		t.Fatal("esc must close the palette")
	}
}

func TestPaletteRunsAction(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "p")
	h.typeText("swdoc")
	h.key("enter")
	if len(h.m.overlays) != 0 || h.m.cur != fake.SessionDocs {
		t.Fatalf("overlays %d cur %s", len(h.m.overlays), h.m.cur)
	}
	// An action that opens another overlay: the new one must survive.
	h.keys("ctrl+\\", "p")
	h.typeText("new")
	h.key("enter")
	if len(h.m.overlays) != 1 {
		t.Fatalf("%d overlays", len(h.m.overlays))
	}
	if _, ok := h.m.overlays[0].(*newSessionOverlay); !ok {
		t.Fatalf("overlay = %T", h.m.overlays[0])
	}
	h.key("esc")
	// Mode switch from the palette.
	h.keys("ctrl+\\", "p")
	h.typeText("swdiff")
	h.key("enter")
	if h.m.mode != view.ModeDiff {
		t.Fatalf("mode = %v", h.m.mode)
	}
	// Steering entries open in Code.
	h.keys("ctrl+\\", "p")
	h.typeText("openagents")
	h.key("enter")
	if h.m.mode != view.ModeCode {
		t.Fatalf("steering action: mode %v", h.m.mode)
	}
}

func TestFuzzyMatch(t *testing.T) {
	for _, c := range []struct {
		title, q string
		ok       bool
	}{
		{"New session", "new", true},
		{"New session", "ns", true},
		{"New session", "nsx", false},
		{"Switch to session docs-sweep", "swdoc", true},
		{"Toggle agent panel", "tap", true},
		{"Zoom focused region", "zfr", true},
		{"Zoom focused region", "rz", false},
	} {
		_, pos, ok := fuzzyMatch(c.title, c.q)
		if ok != c.ok {
			t.Errorf("fuzzyMatch(%q, %q) = %v", c.title, c.q, ok)
		}
		if ok && len(pos) != len(c.q) {
			t.Errorf("positions %v for %q", pos, c.q)
		}
	}
	// Consecutive and word-start matches rank higher.
	s1, _, _ := fuzzyMatch("New session", "new")
	s2, _, _ := fuzzyMatch("Restart session now", "new")
	if s1 <= s2 {
		t.Errorf("scores %d <= %d", s1, s2)
	}
}

func TestNewSessionFlow(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "n")
	o, ok := h.m.overlays[0].(*newSessionOverlay)
	if !ok {
		t.Fatalf("overlay = %T", h.m.overlays[0])
	}
	// An empty task is refused, with a message.
	h.key("ctrl+enter")
	if len(h.m.overlays) != 1 || o.err == "" || !strings.Contains(h.plain(), "Describe the task first") {
		t.Fatal("an empty task must be refused")
	}
	h.typeText("Add rate limiting to the API")
	h.key("enter") // a newline in the task, not a submit
	h.typeText("Keep the public API unchanged.")
	if calls(h, "CreateSession") != 0 {
		t.Fatal("enter must not submit the task")
	}
	out := h.plain()
	for _, want := range []string{"Add rate limiting to the API", "Keep the public API unchanged.", "mpx/add-rate-limiting", "main", "implementer"} {
		if !strings.Contains(out, want) {
			t.Errorf("dialog lacks %q", want)
		}
	}
	// The agent comes from the daemon: Claude Code is the default.
	h.key("tab")
	if o.field != nsAgent || o.agentName() != "claude-code" {
		t.Fatalf("field = %d agent = %q", o.field, o.agentName())
	}
	if out := h.plain(); !strings.Contains(out, "Claude Code") || !strings.Contains(out, "Shell") {
		t.Fatalf("agents not offered:\n%s", out)
	}
	// Choose the role with tab, then arrows.
	h.key("tab")
	if o.field != nsRole {
		t.Fatalf("field = %d", o.field)
	}
	h.key("right")
	h.key("right")
	if o.roleName() != "planner" {
		t.Fatalf("role = %s", o.roleName())
	}
	if !strings.Contains(h.plain(), "Reads the code and writes a plan") {
		t.Fatal("role description not shown")
	}
	// Base branch.
	h.key("tab")
	for range 4 {
		h.key("backspace")
	}
	h.typeText("develop")
	h.shot("newsession-filled")
	h.key("ctrl+enter")
	if len(h.m.overlays) != 0 {
		t.Fatal("dialog must close on start")
	}
	h.settle()
	h.pump()
	h.settle()
	if calls(h, "CreateSession") != 1 {
		t.Fatalf("CreateSession called %d times", calls(h, "CreateSession"))
	}
	s := h.m.session()
	if s == nil || s.Slug != "add-rate-limiting" || s.Role != "planner" || s.Agent != "claude-code" || s.BaseBranch != "develop" {
		t.Fatalf("selected session = %+v", s)
	}
	if !strings.Contains(s.Task, "Keep the public API unchanged.") {
		t.Fatalf("task = %q", s.Task)
	}
	if h.m.focus != regionMain {
		t.Fatalf("focus = %v", h.m.focus)
	}
	if !strings.Contains(h.plain(), "add-rate-limiting") {
		t.Fatal("new session missing from the sidebar")
	}
}

func TestNewSessionCancelAndSpecTask(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.key("ctrl+\\")
	h.key("n")
	h.typeText("something")
	h.key("esc")
	if len(h.m.overlays) != 0 || calls(h, "CreateSession") != 0 {
		t.Fatal("esc must cancel")
	}
	h.keys("ctrl+\\", "n")
	h.key("ctrl+t")
	h.settle()
	if out := h.plain(); !strings.Contains(out, "Implement task 4 of spec token-refresh") {
		t.Fatalf("spec task not filled in:\n%s", out)
	}
}

func TestNewSessionCatalog(t *testing.T) {
	h := newHarness(t, 180, 48)
	if got := len(h.m.roles); got != 6 || h.m.info.Roles != 6 {
		t.Errorf("roles = %d, sidebar count = %d, want the daemon's 6", got, h.m.info.Roles)
	}
	if !strings.Contains(h.plain(), "ROLES") {
		t.Fatal("sidebar has no roles section")
	}
	h.keys("ctrl+\\", "n")
	o := h.m.overlays[0].(*newSessionOverlay)
	if len(o.agents) != 2 || len(o.roles) != 6 {
		t.Fatalf("dialog lists %d agents and %d roles", len(o.agents), len(o.roles))
	}
	// Pick the shell agent and start: the session is created with it.
	h.keys("tab")
	h.key("right")
	if o.agentName() != "shell" {
		t.Fatalf("agent = %q", o.agentName())
	}
	h.keys("shift+tab")
	h.typeText("Try the shell")
	h.key("ctrl+enter")
	h.settle()
	h.pump()
	h.settle()
	if s := h.m.session(); s == nil || s.Agent != "shell" {
		t.Fatalf("selected session = %+v", s)
	}
}

func TestNewSessionRefusesMissingAgent(t *testing.T) {
	h := newHarness(t, 180, 48)
	agents := []protocol.AgentInfo{
		{Name: "claude-code", DisplayName: "Claude Code", Available: false, Detail: "claude was not found on PATH. Install Claude Code from https://code.claude.com", Default: true},
		{Name: "shell", DisplayName: "Shell", Available: true},
	}
	var fresh newSessionOverlay
	fresh.setCatalog(agents, nil)
	if fresh.agentName() != "shell" {
		t.Fatalf("an uninstalled default must give way to an installed agent, got %q", fresh.agentName())
	}
	h.keys("ctrl+\\", "n")
	o := h.m.overlays[0].(*newSessionOverlay)
	o.setCatalog(agents, nil)
	o.agent = 0
	h.typeText("Needs claude")
	h.key("ctrl+enter")
	if len(h.m.overlays) != 1 || !strings.Contains(o.err, "not installed") || calls(h, "CreateSession") != 0 {
		t.Fatalf("err = %q, overlays %d", o.err, len(h.m.overlays))
	}
	if out := h.plain(); !strings.Contains(out, "Claude Code is not installed") || !strings.Contains(out, "Install Claude Code") {
		t.Errorf("the problem and the remedy must be shown:\n%s", out)
	}
}

func TestNewSessionWithoutCatalog(t *testing.T) {
	// A backend that cannot list agents or roles still gets a working dialog.
	h := newHarness(t, 180, 48)
	h.m.agents, h.m.roles = nil, nil
	n, _ := newNewSession(h.m)
	if n.agentName() != "" || n.roleName() != "implementer" {
		t.Errorf("fallback = agent %q role %q", n.agentName(), n.roleName())
	}
}

func TestNoteDismissedUpdatesSidebarBadge(t *testing.T) {
	h := newHarness(t, 180, 48)
	if got := h.m.notes[fake.SessionAuth]; got != 3 {
		t.Fatalf("auth badge = %d, want 3", got)
	}
	h.be.Emit(protocol.TopicNoteDismissed, core.Note{ID: "note_1", SessionID: fake.SessionAuth, Number: 1, Dismissed: true})
	h.pump()
	if got := h.m.notes[fake.SessionAuth]; got != 2 {
		t.Errorf("badge after note.dismissed = %d, want 2", got)
	}
	// Never below zero, even for a repeated event.
	for range 4 {
		h.be.Emit(protocol.TopicNoteDismissed, core.Note{ID: "x", SessionID: fake.SessionAuth, Dismissed: true})
	}
	h.pump()
	if got := h.m.notes[fake.SessionAuth]; got != 0 {
		t.Errorf("badge = %d, want 0", got)
	}
}

func TestToastShowsHints(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.send(view.ToastMsg{
		Tone: kit.ToneSuccess, Title: "Comment queued for auth-refactor", Body: "1 queued",
		Hints: []kit.Hint{{Keys: "ctrl+s", Label: "send now"}, {Keys: "u", Label: "undo"}},
	})
	out := h.plain()
	if !strings.Contains(out, "Comment queued for auth-refactor") || !strings.Contains(out, "send now") || !strings.Contains(out, "undo") {
		t.Fatalf("toast hints missing:\n%s", out)
	}
}

// notRepoBackend is the fake daemon opened in a directory outside git.
type notRepoBackend struct{ *fake.Backend }

func (notRepoBackend) OpenWorkspace(context.Context, string) (core.Workspace, error) {
	return core.Workspace{}, &protocol.Error{Code: protocol.CodeNotARepo, Message: "/tmp/plain is not inside a git repository"}
}

func TestNotARepositoryScreen(t *testing.T) {
	be := fake.New(fake.Empty())
	t.Cleanup(func() { be.Close() })
	m := newModel(Options{Backend: notRepoBackend{be}, WorkspaceDir: "/tmp/plain"}, theme.DarkPlus(), demoProbe)
	m.noPump = true
	h := &harness{t: t, m: m, be: be}
	h.run(m.Init())
	h.send(tea.WindowSizeMsg{Width: 180, Height: 48})
	h.settle()
	out := h.plain()
	for _, want := range []string{"Not a git repository", "/tmp/plain is not inside one", "git init"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "bad_request") || strings.Contains(out, "Could not reach the daemon") {
		t.Errorf("a raw error leaked into the screen:\n%s", out)
	}
	assertRect(t, h.screen(), 180, 48)
	// New session is refused with a reason instead of opening a dialog.
	h.keys("ctrl+\\", "n")
	if len(h.m.overlays) != 0 || !strings.Contains(h.plain(), "No repository here") {
		t.Errorf("new session must explain itself: overlays %d", len(h.m.overlays))
	}
}

// shellSpy records the size a shell pane is opened at.
type shellSpy struct {
	*fake.Backend
	cols, rows int
}

func (s *shellSpy) OpenShell(ctx context.Context, id core.SessionID, cols, rows int) (core.Pane, error) {
	s.cols, s.rows = cols, rows
	return s.Backend.OpenShell(ctx, id, cols, rows)
}

// TestTermShellOpensAtTheMainViewSize: the Term view is built the first time
// the user switches to it, after the last resize, and must still open its
// shell at the main view's size, not at a 20-column default.
func TestTermShellOpensAtTheMainViewSize(t *testing.T) {
	be := &shellSpy{Backend: fake.New()}
	t.Cleanup(func() { be.Close() })
	m := newModel(Options{Backend: be, WorkspaceDir: "/home/user/dev/multi-plex"}, theme.DarkPlus(), demoProbe)
	m.noPump = true
	h := &harness{t: t, m: m, be: be.Backend}
	h.run(m.Init())
	h.send(tea.WindowSizeMsg{Width: 180, Height: 48})
	h.settle()
	h.keys("ctrl+\\", "5")
	h.settle()
	wantCols, wantRows := m.lay.mainInner()
	if be.cols != wantCols || be.rows != wantRows {
		t.Errorf("shell opened at %dx%d, want the main view's %dx%d", be.cols, be.rows, wantCols, wantRows)
	}
}

func TestApprovalLongCommandNeverOverflows(t *testing.T) {
	cmd := "rm -rf " + strings.Repeat("some/long/path/component ", 80) + "&& echo done"
	for _, size := range [][2]int{{180, 48}, {110, 32}, {80, 24}} {
		h := newHarness(t, size[0], size[1])
		req := core.ApprovalRequest{
			ID: "apr_long", SessionID: fake.SessionAPI, Kind: core.ApprovalCommand, Tool: "Bash",
			Command: cmd, Cwd: "/wt", Paths: []string{"a.go", "b.go", "c.go", "d.go"}, Layer: "Layer 4 recursive delete", Rule: "rm-recursive · rules:1",
		}
		h.be.Emit(protocol.TopicApprovalPending, protocol.ApprovalPending{Request: req, Method: "windows-hello"})
		h.pump()
		out := h.plain()
		if !strings.Contains(out, "Approve once") || !strings.Contains(out, "Deny") {
			t.Errorf("%dx%d: buttons are off screen:\n%s", size[0], size[1], out)
		}
		assertRect(t, h.screen(), size[0], size[1])
		if size[1] <= 32 && !strings.Contains(out, "not shown") {
			t.Errorf("%dx%d: a clipped command must say so:\n%s", size[0], size[1], out)
		}
	}
}

func TestRemoveSessionAsksFirst(t *testing.T) {
	h := newHarness(t, 180, 48)
	before := len(h.m.sessions)
	slug := h.m.session().Slug
	h.keys("ctrl+\\", "X")
	if _, ok := h.m.overlays[len(h.m.overlays)-1].(*confirmOverlay); !ok {
		t.Fatalf("overlay = %T", h.m.overlays)
	}
	out := h.plain()
	for _, want := range []string{"Remove " + slug + "?", "worktree", "Remove session", "Cancel"} {
		if !strings.Contains(out, want) {
			t.Errorf("confirm lacks %q:\n%s", want, out)
		}
	}
	h.key("esc")
	if calls(h, "RemoveSession") != 0 || len(h.m.sessions) != before {
		t.Fatal("esc must not remove anything")
	}
	h.keys("ctrl+\\", "X")
	h.key("enter")
	h.settle()
	h.pump()
	if calls(h, "RemoveSession") != 1 {
		t.Fatalf("RemoveSession called %d times", calls(h, "RemoveSession"))
	}
	if len(h.m.sessions) != before-1 || h.m.sessionByID(h.m.cur) == nil && len(h.m.sessions) > 0 {
		t.Errorf("sessions %d (was %d), cur %q", len(h.m.sessions), before, h.m.cur)
	}
}

func TestPaletteShowsEscOnce(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "p")
	if n := strings.Count(h.plain(), "esc close"); n != 1 {
		t.Errorf(`"esc close" appears %d times, want 1`, n)
	}
}

func TestNewSessionNoteWrapsAt80Columns(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.keys("ctrl+\\", "n")
	out := h.plain()
	if !strings.Contains(out, "background.") || strings.Contains(out, "runs in the…") {
		t.Errorf("the setup note must wrap, not truncate:\n%s", out)
	}
	if !strings.Contains(out, "Start session") || !strings.Contains(out, "╰") {
		t.Errorf("the dialog must fit a 24-row terminal, buttons and frame included:\n%s", out)
	}
	assertRect(t, h.screen(), 80, 24)
}

func TestToastStaysInsideTheDrawer(t *testing.T) {
	h := newHarness(t, 80, 24)
	h.keys("ctrl+\\", "a") // open the agent drawer
	if !h.m.lay.drawer {
		t.Skip("no drawer at this size")
	}
	h.send(view.ToastMsg{Tone: kit.ToneSuccess, Title: "Saved", Body: "ok"})
	blocks, xs, _ := h.m.toastRects()
	if len(blocks) == 0 {
		t.Fatal("no toast drawn")
	}
	left := h.m.lay.w - h.m.lay.drawerW
	for i := range blocks {
		if xs[i] <= left || xs[i]+kit.Width(strings.Split(blocks[i], "\n")[0]) >= h.m.lay.w {
			t.Errorf("toast at x=%d width %d straddles the drawer frame at x=%d", xs[i], kit.Width(strings.Split(blocks[i], "\n")[0]), left)
		}
	}
}

func TestApprovalDotIsStaticWithReducedMotion(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.m.th = h.m.th.WithReducedMotion(true)
	for frame := 0; frame < 24; frame++ {
		h.m.frame = frame
		if !h.m.blinkOn() {
			t.Fatalf("approval dot blinks at frame %d with reduced motion", frame)
		}
	}
	if h.m.needsTick() && len(h.m.sessions) == 0 {
		t.Error("nothing animates, yet a tick is armed")
	}
}
