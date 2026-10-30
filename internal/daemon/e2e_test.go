package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// TestEndToEnd drives the daemon the way the TUI and `mpx hook` do: open a
// workspace on a git repository, create a session with the shell adapter,
// type into its pane and watch the echo arrive as pane.frame events, then
// fire a pre-tool hook for `git push --force`, see approval.pending, deny it
// with a message and check the hook response carries that message.
func TestEndToEnd(t *testing.T) {
	h := newHarness(t)

	sess := h.newSession("shell", "e2e shell")
	if sess.Slug != "e2e-shell" || sess.Branch != "mpx/e2e-shell" {
		t.Fatalf("slug/branch = %q/%q", sess.Slug, sess.Branch)
	}
	if sess.AgentPane == "" {
		t.Fatalf("session has no agent pane: %+v", sess)
	}
	wt := filepath.Join(h.paths.WorktreesDir, string(h.ws.ID), "e2e-shell")
	if sess.WorktreePath != wt {
		t.Errorf("worktree = %q, want %q", sess.WorktreePath, wt)
	}
	if _, err := os.Stat(filepath.Join(wt, "README.md")); err != nil {
		t.Errorf("worktree has no checkout: %v", err)
	}
	if br := gitRun(t, wt, "branch", "--show-current"); br != "mpx/e2e-shell" {
		t.Errorf("worktree branch = %q", br)
	}
	if sess.BaseBranch != "main" {
		t.Errorf("base branch = %q, want main", sess.BaseBranch)
	}

	// Input to the pane comes back as frames.
	h.mustCall(protocol.MethodPaneInput, protocol.PaneInputParams{PaneID: sess.AgentPane, Data: []byte("echo hello-$((40+2))-mpx\r")}, nil)
	h.ev.waitScreen(t, sess.AgentPane, "hello-42-mpx")

	var snap protocol.PaneFrame
	h.mustCall(protocol.MethodPaneSnapshot, protocol.PaneRef{PaneID: sess.AgentPane}, &snap)
	if !snap.Full || snap.Cols != 100 || snap.Rows != 20 {
		t.Errorf("snapshot = full %v, %dx%d", snap.Full, snap.Cols, snap.Rows)
	}

	// A destructive command is gated; the user denies it with a message.
	ev := agent.HookEvent{
		Kind: agent.HookPreTool, ToolName: "Bash", ToolClass: agent.ToolShell,
		Command: "git push --force origin main", Cwd: sess.WorktreePath,
	}
	respCh := make(chan agent.HookResponse, 1)
	go func() { respCh <- h.hook(sess.ID, ev) }()

	pending := decode[protocol.ApprovalPending](t, h.ev.wait(t, protocol.TopicApprovalPending, nil))
	if pending.Request.SessionID != sess.ID || pending.Request.Command != ev.Command || pending.Request.Kind != core.ApprovalPush {
		t.Errorf("pending request = %+v", pending.Request)
	}
	if pending.Request.Rule == "" || pending.Request.Layer == "" {
		t.Errorf("pending request lacks rule/layer: %+v", pending.Request)
	}
	if pending.Method != "typed-code" || pending.Code == "" {
		t.Errorf("pending method/code = %q/%q", pending.Method, pending.Code)
	}
	blocked := h.waitState(sess.ID, core.StateBlocked)
	if blocked.StateDetail != "waiting for approval" {
		t.Errorf("blocked detail = %q", blocked.StateDetail)
	}
	var list []core.ApprovalRequest
	h.mustCall(protocol.MethodApprovalList, protocol.Empty{}, &list)
	if len(list) != 1 || list[0].ID != pending.Request.ID {
		t.Errorf("approval.list = %+v", list)
	}

	var dec core.ApprovalDecision
	h.mustCall(protocol.MethodApprovalRespond, protocol.ApprovalRespondParams{
		RequestID: pending.Request.ID, Outcome: core.Denied, Message: "use mpx push instead",
	}, &dec)
	if dec.Outcome != core.Denied || dec.Message != "use mpx push instead" {
		t.Errorf("decision = %+v", dec)
	}
	select {
	case resp := <-respCh:
		if resp.Decision != agent.Deny || !strings.Contains(resp.Reason, "use mpx push instead") {
			t.Errorf("hook response = %+v, want deny carrying the message", resp)
		}
	case <-time.After(waitFor):
		t.Fatal("hook did not return after the denial")
	}
	h.ev.wait(t, protocol.TopicApprovalDone, nil)
	h.waitState(sess.ID, core.StateWorking)

	// The decision was audited and counted.
	var audit []struct {
		Decision core.ApprovalDecision `json:"Decision"`
	}
	h.mustCall(protocol.MethodApprovalAudit, protocol.SessionRef{SessionID: sess.ID}, &audit)
	if len(audit) != 1 || audit[0].Decision.Outcome != core.Denied {
		t.Errorf("audit = %+v", audit)
	}
	if n := h.detail(sess.ID).Approvals; n != 1 {
		t.Errorf("approvals last hour = %d, want 1", n)
	}
}

// TestHookGateApprove approves a gated command with the typed code.
func TestHookGateApprove(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("shell", "approve me")
	ev := agent.HookEvent{
		Kind: agent.HookPreTool, ToolName: "Bash", ToolClass: agent.ToolShell,
		Command: "git reset --hard HEAD~3 && git push --force", Cwd: sess.WorktreePath,
	}
	respCh := make(chan agent.HookResponse, 1)
	go func() { respCh <- h.hook(sess.ID, ev) }()
	p := decode[protocol.ApprovalPending](t, h.ev.wait(t, protocol.TopicApprovalPending, nil))

	// A wrong code does not approve.
	err := h.call(protocol.MethodApprovalRespond, protocol.ApprovalRespondParams{RequestID: p.Request.ID, Outcome: core.Approved, Code: "AAAAAA"}, nil)
	if err == nil {
		t.Fatal("a wrong code was accepted")
	}
	var dec core.ApprovalDecision
	h.mustCall(protocol.MethodApprovalRespond, protocol.ApprovalRespondParams{RequestID: p.Request.ID, Outcome: core.Approved, Code: p.Code}, &dec)
	if dec.Outcome != core.Approved || dec.Method != "typed-code" {
		t.Errorf("decision = %+v", dec)
	}
	select {
	case resp := <-respCh:
		if resp.Decision != agent.Allow {
			t.Errorf("hook response = %+v, want allow", resp)
		}
	case <-time.After(waitFor):
		t.Fatal("hook did not return after approval")
	}
	if err := h.call(protocol.MethodApprovalRespond, protocol.ApprovalRespondParams{RequestID: "apr_nope", Outcome: core.Denied}, nil); err == nil {
		t.Error("responding to an unknown request succeeded")
	}
}

// TestHookWriteOutsideWorktreeGated checks the Layer 2 path of the gate.
func TestHookWriteOutsideWorktreeGated(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("shell", "paths")
	if runtimeIsWindows() {
		t.Skip("path example is POSIX")
	}
	outside := "/opt/mpx-test/file.txt"
	ev := agent.HookEvent{Kind: agent.HookPreTool, ToolName: "Write", ToolClass: agent.ToolWrite, Paths: []string{outside}, Cwd: sess.WorktreePath}
	respCh := make(chan agent.HookResponse, 1)
	go func() { respCh <- h.hook(sess.ID, ev) }()
	p := decode[protocol.ApprovalPending](t, h.ev.wait(t, protocol.TopicApprovalPending, nil))
	if p.Request.Kind != core.ApprovalFileWrite {
		t.Errorf("kind = %q", p.Request.Kind)
	}
	h.mustCall(protocol.MethodApprovalRespond, protocol.ApprovalRespondParams{RequestID: p.Request.ID, Outcome: core.Denied}, nil)
	if resp := <-respCh; resp.Decision != agent.Deny {
		t.Errorf("response = %+v", resp)
	}

	// Inside the worktree is allowed without a prompt.
	in := agent.HookEvent{Kind: agent.HookPreTool, ToolName: "Write", ToolClass: agent.ToolWrite, Paths: []string{filepath.Join(sess.WorktreePath, "new.txt")}, Cwd: sess.WorktreePath}
	if resp := h.hook(sess.ID, in); resp.Decision != agent.Allow {
		t.Errorf("write inside worktree: %+v", resp)
	}
}

func runtimeIsWindows() bool { return os.PathSeparator == '\\' }

// TestPreToolUnknownSession fails closed.
func TestPreToolUnknownSession(t *testing.T) {
	h := newHarness(t)
	resp := h.hook("ses_unknown", agent.HookEvent{Kind: agent.HookPreTool, ToolName: "Read", ToolClass: agent.ToolRead})
	if resp.Decision != agent.Deny {
		t.Errorf("unknown session pre-tool = %+v, want deny", resp)
	}
	if resp := h.hook("ses_unknown", agent.HookEvent{Kind: agent.HookStop}); resp.Decision != "" {
		t.Errorf("unknown session stop = %+v, want empty", resp)
	}
}

// TestHookCancelledWhenClientGoesAway: an approval wait ends, denied, when the
// hook process disconnects.
func TestHookCancelledWhenClientGoesAway(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("shell", "cancel")
	hc := h.dial()
	done := make(chan struct{})
	go func() {
		defer close(done)
		var resp agent.HookResponse
		_ = hc.Call(t.Context(), protocol.MethodHook, protocol.HookParams{SessionID: sess.ID, Event: agent.HookEvent{
			Kind: agent.HookPreTool, ToolName: "Bash", ToolClass: agent.ToolShell, Command: "terraform destroy",
		}}, &resp)
	}()
	h.ev.wait(t, protocol.TopicApprovalPending, nil)
	hc.Close()
	<-done
	h.ev.wait(t, protocol.TopicApprovalDone, func(raw json.RawMessage) bool {
		return decode[core.ApprovalDecision](t, raw).Outcome == core.Denied
	})
	h.waitState(sess.ID, core.StateWorking)
	if out, err := exec.LookPath("git"); err != nil || out == "" {
		t.Skip("git missing")
	}
}
