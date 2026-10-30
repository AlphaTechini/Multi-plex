package app

import (
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend/fake"
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
	h := newHarness(t, 180, 48)
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
	h := newHarness(t, 180, 48, fake.WithApprovalDelay(400*time.Millisecond))
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
	if strings.Contains(h.plain(), "session_revoke.json") {
		t.Fatal("paths must be collapsed at first")
	}
	h.key("l")
	if !strings.Contains(h.plain(), "session_revoke.json") {
		t.Fatal("l must list all paths")
	}
	assertRect(t, h.screen(), 180, 48)
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
	// Choose the role with tab, then arrows.
	h.key("tab")
	if o.field != nsRole {
		t.Fatalf("field = %d", o.field)
	}
	h.key("left")
	if newRoles[o.role].name != "planner" {
		t.Fatalf("role = %s", newRoles[o.role].name)
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
	if s == nil || s.Slug != "add-rate-limiting" || s.Role != "planner" || s.BaseBranch != "develop" {
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
