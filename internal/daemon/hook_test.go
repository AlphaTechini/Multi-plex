package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/agent/claudecode"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

func TestPermissionPrefixMatchesAdapter(t *testing.T) {
	if permissionPrefix != claudecode.PermissionPrefix {
		t.Fatalf("permissionPrefix %q != claudecode.PermissionPrefix %q", permissionPrefix, claudecode.PermissionPrefix)
	}
}

func TestHookAllowFastPath(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "fast")
	h.waitState(sess.ID, core.StateStarting)

	read := agent.HookEvent{Kind: agent.HookPreTool, ToolName: "Read", ToolClass: agent.ToolRead, Paths: []string{filepath.Join(sess.WorktreePath, "README.md")}, Cwd: sess.WorktreePath}
	start := time.Now()
	const n = 50
	for i := 0; i < n; i++ {
		if resp := h.hook(sess.ID, read); resp.Decision != agent.Allow {
			t.Fatalf("read: %+v", resp)
		}
	}
	t.Logf("pre-tool allow: %s per call (client round trip, includes -race overhead)", time.Since(start)/n)
	if got := h.detail(sess.ID).Session.State; got != core.StateWorking {
		t.Errorf("state after pre-tool = %s, want working", got)
	}
	// No checkpoint for a non-shell tool.
	if cps := h.detail(sess.ID).Checkpoints; cps != 0 {
		t.Errorf("checkpoints = %d after reads", cps)
	}

	// A harmless shell command is allowed and checkpointed first.
	ls := agent.HookEvent{Kind: agent.HookPreTool, ToolName: "Bash", ToolClass: agent.ToolShell, Command: "ls -la", Cwd: sess.WorktreePath}
	if resp := h.hook(sess.ID, ls); resp.Decision != agent.Allow {
		t.Fatalf("ls: %+v", resp)
	}
	if cps := h.detail(sess.ID).Checkpoints; cps != 1 {
		t.Fatalf("checkpoints after first shell command = %d, want 1", cps)
	}
	// Right after, with no write in between, the next shell command reuses it.
	h.hook(sess.ID, ls)
	if cps := h.detail(sess.ID).Checkpoints; cps != 1 {
		t.Errorf("checkpoints = %d, want 1 (skipped within 2s without a write)", cps)
	}
	// After a write, the next shell command gets a new checkpoint.
	writeFile(t, filepath.Join(sess.WorktreePath, "new.txt"), "x\n")
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookPostTool, ToolName: "Write", ToolClass: agent.ToolWrite})
	h.hook(sess.ID, ls)
	var cps []core.Checkpoint
	h.mustCall(protocol.MethodCheckpointList, protocol.SessionRef{SessionID: sess.ID}, &cps)
	if len(cps) != 2 {
		t.Fatalf("checkpoints = %d, want 2", len(cps))
	}
	if want := "refs/multiplex/checkpoints/" + string(sess.ID) + "/2"; cps[1].Ref != want {
		t.Errorf("ref = %q, want %q", cps[1].Ref, want)
	}
	if got := gitRun(t, h.repo, "rev-parse", cps[1].Ref); got != cps[1].Commit {
		t.Errorf("ref points at %s, want %s", got, cps[1].Commit)
	}
}

func TestHookStateMachine(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "states")

	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookPromptSubmit})
	h.waitState(sess.ID, core.StateWorking)

	// Claude's own permission prompt is the immediate blocked signal.
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookNotification, Message: permissionPrefix + " waiting for permission to use Bash"})
	s := h.waitState(sess.ID, core.StateBlocked)
	if s.StateDetail != "waiting for permission" {
		t.Errorf("detail = %q", s.StateDetail)
	}
	// The tool ran: working again.
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookPostTool, ToolName: "Bash", ToolClass: agent.ToolShell})
	h.waitState(sess.ID, core.StateWorking)

	// Other notifications keep the state.
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookNotification, Message: "Claude Code login successful"})
	if got := h.detail(sess.ID).Session.State; got != core.StateWorking {
		t.Errorf("state after plain notification = %s", got)
	}
	// An idle notification means done.
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookNotification, Message: "Claude is waiting for your input"})
	h.waitState(sess.ID, core.StateDone)

	// A new prompt after a quiet period goes back to working (an interrupt
	// produces no Stop event).
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookPromptSubmit})
	h.waitState(sess.ID, core.StateWorking)

	// Stop: done, unseen, a turn-end checkpoint and a diff.changed event.
	writeFile(t, filepath.Join(sess.WorktreePath, "work.go"), "package main\n")
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookStop})
	done := h.waitState(sess.ID, core.StateDone)
	if !done.Unseen {
		t.Error("done session is not marked unseen")
	}
	h.ev.wait(t, protocol.TopicDiffChanged, nil)
	deadline := time.Now().Add(waitFor)
	for h.detail(sess.ID).Checkpoints == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	var cps []core.Checkpoint
	h.mustCall(protocol.MethodCheckpointList, protocol.SessionRef{SessionID: sess.ID}, &cps)
	if len(cps) == 0 || cps[len(cps)-1].Reason != "turn end" {
		t.Errorf("checkpoints = %+v, want a turn-end checkpoint", cps)
	}
	h.mustCall(protocol.MethodSessionSeen, protocol.SessionRef{SessionID: sess.ID}, nil)
	if h.detail(sess.ID).Session.Unseen {
		t.Error("session.seen did not clear unseen")
	}
	// Changes are listed.
	if ch := h.detail(sess.ID).Changes; len(ch) != 1 || ch[0].Path != "work.go" {
		t.Errorf("changes = %+v", ch)
	}

	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookSessionEnd})
	h.waitState(sess.ID, core.StateIdle)
}

func TestHookPostToolEditEvent(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "follow")
	h.mustCall(protocol.MethodSessionFollow, protocol.SessionFollowParams{SessionID: sess.ID, On: true}, nil)
	if !h.detail(sess.ID).Session.Follow {
		t.Error("follow not set")
	}
	abs := filepath.Join(sess.WorktreePath, "src", "a.go")
	writeFile(t, abs, "package a\n")
	h.hook(sess.ID, agent.HookEvent{
		Kind: agent.HookPostTool, ToolName: "Edit", ToolClass: agent.ToolWrite,
		Edits: []agent.EditRange{{Path: abs, StartLine: 3, EndLine: 9}},
	})
	e := decode[core.EditEvent](t, h.ev.wait(t, protocol.TopicEdit, nil))
	if e.SessionID != sess.ID || e.Path != "src/a.go" || e.StartLine != 3 || e.EndLine != 9 {
		t.Errorf("edit event = %+v", e)
	}
	start := time.Now()
	h.ev.wait(t, protocol.TopicDiffChanged, nil)
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("diff.changed took %s", d)
	}
}

func TestSessionStartTypesTask(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "typed task")
	h.ev.waitScreen(t, sess.AgentPane, "helper-agent-ready")
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookSessionStart})
	h.ev.waitScreen(t, sess.AgentPane, `helper-got:"task typed task"`)
	// Only once.
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookSessionStart})
	time.Sleep(taskTypeDelay + 300*time.Millisecond)
	if n := strings.Count(h.ev.screenText(sess.AgentPane), "helper-got"); n != 1 {
		t.Errorf("task typed %d times", n)
	}
}

func TestInitialPromptTaskIsNotTyped(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("prompt-helper", "from the command line")
	h.ev.waitScreen(t, sess.AgentPane, `helper-prompt:"task from the command line"`)
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookSessionStart})
	time.Sleep(taskTypeDelay + 300*time.Millisecond)
	if n := strings.Count(h.ev.screenText(sess.AgentPane), "helper-got"); n != 0 {
		t.Errorf("task typed %d times for an InitialPrompt adapter", n)
	}

	// A restart is not a first launch: the task is not passed again.
	var again core.Session
	h.mustCall(protocol.MethodSessionRestart, protocol.SessionRef{SessionID: sess.ID}, &again)
	h.ev.waitScreen(t, again.AgentPane, `helper-prompt:""`)
}

func TestCheckpointRestore(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "restore")
	file := filepath.Join(sess.WorktreePath, "README.md")
	writeFile(t, file, "version one\n")
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookPreTool, ToolName: "Bash", ToolClass: agent.ToolShell, Command: "ls"})
	writeFile(t, file, "version two\n")
	writeFile(t, filepath.Join(sess.WorktreePath, "extra.txt"), "x\n")

	h.mustCall(protocol.MethodCheckpointRestore, protocol.CheckpointRestoreParams{SessionID: sess.ID, N: 1}, nil)
	b, _ := os.ReadFile(file)
	if string(b) != "version one\n" {
		t.Errorf("README after restore = %q", b)
	}
	if _, err := os.Stat(filepath.Join(sess.WorktreePath, "extra.txt")); err == nil {
		t.Error("file created after the checkpoint survived the restore")
	}
	var cps []core.Checkpoint
	h.mustCall(protocol.MethodCheckpointList, protocol.SessionRef{SessionID: sess.ID}, &cps)
	if len(cps) != 2 || cps[1].Reason != "before restore" {
		t.Fatalf("checkpoints = %+v, want a 'before restore' checkpoint", cps)
	}
	// The restore itself is reversible.
	h.mustCall(protocol.MethodCheckpointRestore, protocol.CheckpointRestoreParams{SessionID: sess.ID, N: 2}, nil)
	b, _ = os.ReadFile(file)
	if string(b) != "version two\n" {
		t.Errorf("README after undoing the restore = %q", b)
	}
	if err := h.call(protocol.MethodCheckpointRestore, protocol.CheckpointRestoreParams{SessionID: sess.ID, N: 99}, nil); err == nil {
		t.Error("restoring a missing checkpoint succeeded")
	}
}

func TestDiffAndFileRead(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "diffing")
	writeFile(t, filepath.Join(sess.WorktreePath, "README.md"), "hello\nmore\n")
	writeFile(t, filepath.Join(sess.WorktreePath, "bin.dat"), "a\x00b")

	var res protocol.DiffResult
	h.mustCall(protocol.MethodDiffGet, protocol.SessionRef{SessionID: sess.ID}, &res)
	if !strings.Contains(res.Raw, "+more") || len(res.Changes) != 2 {
		t.Errorf("diff = %q, changes = %+v", res.Raw, res.Changes)
	}
	if res.Base == "" || res.Head == "" || res.Base != res.Head {
		t.Errorf("base/head = %q/%q (no commits yet, so they match)", res.Base, res.Head)
	}

	var f protocol.FileReadResult
	h.mustCall(protocol.MethodFileRead, protocol.FileReadParams{SessionID: sess.ID, Path: "README.md"}, &f)
	if f.Content != "hello\nmore\n" || f.Path != "README.md" {
		t.Errorf("file = %+v", f)
	}
	h.mustCall(protocol.MethodFileRead, protocol.FileReadParams{SessionID: sess.ID, Path: "README.md", Rev: res.Base}, &f)
	if f.Content != "hello\n" {
		t.Errorf("file at base = %+v", f)
	}
	h.mustCall(protocol.MethodFileRead, protocol.FileReadParams{SessionID: sess.ID, Path: "bin.dat"}, &f)
	if !f.Binary || f.Content != "" {
		t.Errorf("binary file = %+v", f)
	}
	for _, bad := range []string{"../secret", "/etc/passwd", "a/../../x", ""} {
		if err := h.call(protocol.MethodFileRead, protocol.FileReadParams{SessionID: sess.ID, Path: bad}, nil); err == nil {
			t.Errorf("file.read %q succeeded", bad)
		}
	}
}

// TestDiffPollerPublishes: edits made by a working session are noticed by the
// background poll even without a hook.
func TestDiffPollerPublishes(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "poll")
	h.hook(sess.ID, agent.HookEvent{Kind: agent.HookPromptSubmit})
	h.waitState(sess.ID, core.StateWorking)
	writeFile(t, filepath.Join(sess.WorktreePath, "polled.txt"), "x\n")
	start := time.Now()
	h.ev.wait(t, protocol.TopicDiffChanged, nil)
	t.Logf("poller noticed the change after %s", time.Since(start))
}

func TestHookEventJSON(t *testing.T) {
	// The hook params the CLI sends survive the wire: tool input is a map.
	ev := agent.HookEvent{Kind: agent.HookPreTool, ToolInput: map[string]any{"command": "ls"}}
	b, _ := json.Marshal(protocol.HookParams{SessionID: "s", Event: ev})
	var back protocol.HookParams
	if err := json.Unmarshal(b, &back); err != nil || back.Event.ToolInput["command"] != "ls" {
		t.Errorf("round trip: %v %+v", err, back)
	}
}
