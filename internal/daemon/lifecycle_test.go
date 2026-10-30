package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	_ "github.com/alphatechini/multi-plex/internal/agent/claudecode"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

func TestSessionLifecycle(t *testing.T) {
	h := newHarness(t)
	s1 := h.newSession("helper", "same name")
	s2 := h.newSession("helper", "same name")
	if s1.Slug != "same-name" || s2.Slug != "same-name-2" || s2.Branch != "mpx/same-name-2" {
		t.Errorf("slugs = %q, %q", s1.Slug, s2.Slug)
	}
	if s1.Role != "implementer" || s1.Agent != "helper" {
		t.Errorf("role/agent = %q/%q", s1.Role, s1.Agent)
	}

	// Each session has its own config dir outside its worktree, and the env
	// of the agent carries the session variables and no credentials.
	if _, err := os.Stat(filepath.Join(h.paths.DataDir, "sessions", string(s1.ID))); err != nil {
		t.Errorf("no per-session config dir: %v", err)
	}

	// session.created events were published, and list works.
	h.ev.wait(t, protocol.TopicSession, func(raw json.RawMessage) bool { return decode[core.Session](t, raw).ID == s2.ID })
	var list []core.Session
	h.mustCall(protocol.MethodSessionList, protocol.SessionListParams{WorkspaceID: h.ws.ID}, &list)
	if len(list) != 2 || list[0].ID != s1.ID {
		t.Errorf("list = %+v", list)
	}

	// A shell pane opens in the worktree.
	var pn core.Pane
	h.mustCall(protocol.MethodPaneOpen, protocol.PaneOpenParams{SessionID: s1.ID, Cols: 80, Rows: 10}, &pn)
	if pn.Kind != core.PaneShell || pn.SessionID != s1.ID || pn.ID == "" {
		t.Fatalf("pane = %+v", pn)
	}
	h.mustCall(protocol.MethodPaneInput, protocol.PaneInputParams{PaneID: pn.ID, Data: []byte("pwd; echo \"port=$MPX_PORT_BASE sid=$MPX_SESSION_ID\"\r")}, nil)
	screen := h.ev.waitScreen(t, pn.ID, "sid="+string(s1.ID))
	// The prompt and pwd wrap at 80 columns, so check the unwrapped parts.
	if !strings.Contains(screen, "worktrees") || !strings.Contains(screen, "same-") {
		t.Errorf("shell is not in the worktree:\n%s", screen)
	}
	if !strings.Contains(screen, "port=20000") { // first 100-port block
		t.Errorf("no port base in the shell env:\n%s", screen)
	}
	h.mustCall(protocol.MethodPaneResize, protocol.PaneResizeParams{PaneID: pn.ID, Cols: 60, Rows: 8}, nil)
	var snap protocol.PaneFrame
	h.mustCall(protocol.MethodPaneSnapshot, protocol.PaneRef{PaneID: pn.ID}, &snap)
	if snap.Cols != 60 || snap.Rows != 8 {
		t.Errorf("snapshot after resize = %dx%d", snap.Cols, snap.Rows)
	}
	h.mustCall(protocol.MethodPaneClose, protocol.PaneRef{PaneID: pn.ID}, nil)
	h.ev.wait(t, protocol.TopicPaneExited, func(raw json.RawMessage) bool { return decode[core.Pane](t, raw).ID == pn.ID })
	if err := h.call(protocol.MethodPaneSnapshot, protocol.PaneRef{PaneID: pn.ID}, nil); err == nil {
		t.Error("snapshot of a closed pane succeeded")
	}

	// Stop kills the agent and keeps the worktree.
	h.mustCall(protocol.MethodSessionStop, protocol.SessionRef{SessionID: s1.ID}, nil)
	st := h.waitState(s1.ID, core.StateIdle)
	if st.AgentPane != "" || st.StateDetail != "stopped" {
		t.Errorf("after stop: %+v", st)
	}
	if _, err := os.Stat(s1.WorktreePath); err != nil {
		t.Errorf("stop removed the worktree: %v", err)
	}

	// Restart relaunches in the same worktree.
	var re core.Session
	h.mustCall(protocol.MethodSessionRestart, protocol.SessionRef{SessionID: s1.ID}, &re)
	if re.AgentPane == "" || re.WorktreePath != s1.WorktreePath || re.State != core.StateStarting {
		t.Errorf("after restart: %+v", re)
	}
	h.ev.waitScreen(t, re.AgentPane, "helper-agent-ready")

	// Remove deletes the worktree, branch and records.
	h.mustCall(protocol.MethodSessionRemove, protocol.SessionRef{SessionID: s1.ID}, nil)
	h.ev.wait(t, protocol.TopicSessionRemoved, nil)
	if _, err := os.Stat(s1.WorktreePath); err == nil {
		t.Error("worktree still exists after remove")
	}
	if out := gitRun(t, h.repo, "branch", "--list", "mpx/same-name"); out != "" {
		t.Errorf("branch still exists: %q", out)
	}
	if _, err := h.st.GetSession(t.Context(), s1.ID); err == nil {
		t.Error("session still in the store")
	}
	if err := h.call(protocol.MethodSessionGet, protocol.SessionRef{SessionID: s1.ID}, nil); err == nil {
		t.Error("session.get of a removed session succeeded")
	}
}

func TestAgentExitMovesSessionState(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("shell", "exits")
	h.mustCall(protocol.MethodPaneInput, protocol.PaneInputParams{PaneID: sess.AgentPane, Data: []byte("exit 3\r")}, nil)
	s := h.waitState(sess.ID, core.StateError)
	if s.StateDetail != "agent exited with code 3" {
		t.Errorf("detail = %q", s.StateDetail)
	}
	exited := decode[core.Pane](t, h.ev.wait(t, protocol.TopicPaneExited, nil))
	if exited.ID != sess.AgentPane || !exited.Exited || exited.ExitCode != 3 {
		t.Errorf("pane.exited = %+v", exited)
	}

	sess2 := h.newSession("shell", "exits cleanly")
	h.mustCall(protocol.MethodPaneInput, protocol.PaneInputParams{PaneID: sess2.AgentPane, Data: []byte("exit 0\r")}, nil)
	h.waitState(sess2.ID, core.StateIdle)
}

func TestMissingAgentBinaryIsAnErrorState(t *testing.T) {
	h := newHarness(t)
	t.Setenv("PATH", t.TempDir()) // git was resolved already; claude is not on this PATH
	var sess core.Session
	h.mustCall(protocol.MethodSessionCreate, protocol.SessionCreateParams{WorkspaceID: h.ws.ID, Task: "needs claude", Agent: "claude-code"}, &sess)
	if sess.State != core.StateError {
		t.Fatalf("state = %s, want error", sess.State)
	}
	if !strings.Contains(sess.StateDetail, "claude") || !strings.Contains(strings.ToLower(sess.StateDetail), "install") {
		t.Errorf("detail does not explain how to fix it: %q", sess.StateDetail)
	}
	if sess.AgentPane != "" {
		t.Errorf("agent pane set for a failed launch")
	}
	// The worktree exists, and the daemon is fine.
	if _, err := os.Stat(sess.WorktreePath); err != nil {
		t.Error(err)
	}
	h.mustCall(protocol.MethodSessionRemove, protocol.SessionRef{SessionID: sess.ID}, nil)
}

func TestCreateValidation(t *testing.T) {
	h := newHarness(t)
	for name, p := range map[string]protocol.SessionCreateParams{
		"unknown workspace": {WorkspaceID: "ws_nope", Task: "x", Agent: "shell"},
		"unknown agent":     {WorkspaceID: h.ws.ID, Task: "x", Agent: "emacs"},
		"unknown role":      {WorkspaceID: h.ws.ID, Task: "x", Agent: "shell", Role: "wizard"},
		"unknown base":      {WorkspaceID: h.ws.ID, Task: "x", Agent: "shell", BaseBranch: "no-such-branch"},
	} {
		if err := h.call(protocol.MethodSessionCreate, p, nil); err == nil {
			t.Errorf("%s: create succeeded", name)
		}
	}
	if err := h.call(protocol.MethodWorkspaceOpen, protocol.WorkspaceOpenParams{Dir: t.TempDir()}, nil); err == nil {
		t.Error("opening a non-repository succeeded")
	}
	if err := h.call("no.such.method", protocol.Empty{}, nil); err == nil {
		t.Error("unknown method succeeded")
	}
	var list []core.Workspace
	h.mustCall(protocol.MethodWorkspaceList, protocol.Empty{}, &list)
	if len(list) != 1 || list[0].Root != h.repo {
		t.Errorf("workspaces = %+v", list)
	}
	var ws2 core.Workspace
	h.mustCall(protocol.MethodWorkspaceOpen, protocol.WorkspaceOpenParams{Dir: filepath.Join(h.repo, ".")}, &ws2)
	if ws2.ID != h.ws.ID {
		t.Error("opening the same repository twice made a second workspace")
	}
}

func TestBaseBranchOption(t *testing.T) {
	h := newHarness(t)
	gitRun(t, h.repo, "branch", "develop")
	var s core.Session
	h.mustCall(protocol.MethodSessionCreate, protocol.SessionCreateParams{WorkspaceID: h.ws.ID, Task: "x", Agent: "shell", BaseBranch: "develop"}, &s)
	if s.BaseBranch != "develop" {
		t.Errorf("base = %q", s.BaseBranch)
	}
}

// TestDaemonRestartKeepsSessions: sessions that were active become idle, and
// session.restart relaunches the agent in the existing worktree.
func TestDaemonRestartKeepsSessions(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "survivor")
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookPromptSubmit})
	h.waitState(sess.ID, core.StateWorking)
	h.mcp(sess.ID, protocol.ToolNoteToUser, protocol.NoteToUserArgs{Title: "kept"})
	gone := h.newSession("helper", "finished")
	h.hook(gone.ID, agent.HookEvent{Kind: agent.HookStop})
	h.waitState(gone.ID, core.StateDone)

	// Stop the daemon without closing the store (the next daemon reuses it).
	st, paths, repo := h.st, h.paths, h.repo
	h.c.Close()
	h.cancel()
	<-h.served
	h.cancel = nil
	// The first daemon closed the memory store; memory.Store.Close is a no-op.

	h2 := &harness{t: t, st: st, paths: paths, repo: repo}
	dp, b := newDeps(t, st, paths)
	h2.broker = b
	h2.start(New(dp))
	t.Cleanup(h2.stop)
	h2.ws = h.ws

	got := h2.detail(sess.ID)
	if got.Session.State != core.StateIdle || got.Session.StateDetail != "agent stopped when the daemon restarted" || got.Session.AgentPane != "" {
		t.Errorf("restored session = %+v", got.Session)
	}
	if len(got.Notes) != 1 {
		t.Errorf("notes lost: %+v", got.Notes)
	}
	if d := h2.detail(gone.ID).Session; d.State != core.StateDone || !d.Unseen {
		t.Errorf("a done session should stay done and unseen: %+v", d)
	}

	var re core.Session
	h2.mustCall(protocol.MethodSessionRestart, protocol.SessionRef{SessionID: sess.ID}, &re)
	if re.AgentPane == "" {
		t.Fatalf("restart did not launch: %+v", re)
	}
	h2.ev.waitScreen(t, re.AgentPane, "helper-agent-ready")
	// New sessions in the restored daemon still get unique slugs.
	var again core.Session
	h2.mustCall(protocol.MethodSessionCreate, protocol.SessionCreateParams{WorkspaceID: h.ws.ID, Name: "survivor", Agent: "shell"}, &again)
	if again.Slug != "survivor-2" {
		t.Errorf("slug after restart = %q", again.Slug)
	}
}

func TestShutdownMethod(t *testing.T) {
	h := newHarness(t)
	h.newSession("helper", "bye")
	h.mustCall(protocol.MethodShutdown, protocol.Empty{}, nil)
	select {
	case err := <-h.served:
		if err != nil {
			t.Errorf("Serve returned %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("daemon did not shut down")
	}
	h.cancel = nil
	select {
	case <-h.d.Done():
	default:
		t.Error("Done not closed after shutdown")
	}
}

func TestHelloAndSubscribeFilter(t *testing.T) {
	h := newHarness(t)
	res, err := h.c.Hello(t.Context(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if res.Protocol != protocol.Version || res.Version != "test" || res.PID != os.Getpid() || res.Approver != "typed-code" {
		t.Errorf("hello = %+v", res)
	}

	// A second client subscribed only to toasts sees no session events.
	c2 := h.dial()
	defer c2.Close()
	log2 := newEventLog(c2)
	if err := c2.Subscribe(t.Context(), protocol.TopicNote); err != nil {
		t.Fatal(err)
	}
	sess := h.newSession("helper", "filtered")
	h.mcp(sess.ID, protocol.ToolNoteToUser, protocol.NoteToUserArgs{Title: "only notes"})
	log2.wait(t, protocol.TopicNote, nil)
	log2.mu.Lock()
	defer log2.mu.Unlock()
	for _, ev := range log2.events {
		if ev.Topic != protocol.TopicNote {
			t.Errorf("filtered subscriber got %s", ev.Topic)
		}
	}
}

// TestLateSubscriberSeesPendingApproval: a TUI that attaches while an approval
// waits is told about it, with the typed code.
func TestLateSubscriberSeesPendingApproval(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "late")
	go func() {
		var resp agent.HookResponse
		_ = h.c.Call(context.Background(), protocol.MethodHook, protocol.HookParams{SessionID: sess.ID, Event: agent.HookEvent{
			Kind: agent.HookPreTool, ToolName: "Bash", ToolClass: agent.ToolShell, Command: "terraform destroy",
		}}, &resp)
	}()
	first := decode[protocol.ApprovalPending](t, h.ev.wait(t, protocol.TopicApprovalPending, nil))

	c2 := h.dial()
	defer c2.Close()
	log2 := newEventLog(c2)
	if err := c2.Subscribe(t.Context()); err != nil {
		t.Fatal(err)
	}
	late := decode[protocol.ApprovalPending](t, log2.wait(t, protocol.TopicApprovalPending, nil))
	if late.Request.ID != first.Request.ID || late.Code != first.Code || late.Code == "" {
		t.Errorf("late = %+v, first = %+v", late, first)
	}
	h.mustCall(protocol.MethodApprovalRespond, protocol.ApprovalRespondParams{RequestID: first.Request.ID, Outcome: core.Denied}, nil)
}
