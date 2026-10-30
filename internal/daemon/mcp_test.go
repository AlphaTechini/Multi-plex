package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

func TestMCPNoteAndProgress(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "mcp notes")

	res := h.mcp(sess.ID, protocol.ToolNoteToUser, protocol.NoteToUserArgs{
		Title: "Changing Session.Refresh", Body: "Callers need updating.", Severity: "warning",
		Refs: []string{"src/auth/session.go:L40-L52"},
	})
	if res.IsError || !strings.Contains(res.Text, "#1") {
		t.Fatalf("note_to_user = %+v", res)
	}
	n := decode[core.Note](t, h.ev.wait(t, protocol.TopicNote, nil))
	if n.Number != 1 || n.Severity != core.SeverityWarning || n.Title != "Changing Session.Refresh" || len(n.Refs) != 1 {
		t.Errorf("note.added = %+v", n)
	}
	// A decision-needed card does not change the session state.
	h.hook(sess.ID, agentPrompt())
	h.mcp(sess.ID, protocol.ToolNoteToUser, protocol.NoteToUserArgs{Title: "Which db?", Severity: "decision-needed"})
	if got := h.detail(sess.ID).Session.State; got != core.StateWorking {
		t.Errorf("state after decision-needed note = %s", got)
	}
	if bad := h.mcp(sess.ID, protocol.ToolNoteToUser, protocol.NoteToUserArgs{Title: "x", Severity: "loud"}); !bad.IsError {
		t.Error("bad severity accepted")
	}
	if bad := h.mcp(sess.ID, protocol.ToolNoteToUser, protocol.NoteToUserArgs{}); !bad.IsError {
		t.Error("empty title accepted")
	}
	var notes []core.Note
	h.mustCall(protocol.MethodNoteList, protocol.SessionRef{SessionID: sess.ID}, &notes)
	if len(notes) != 2 || notes[1].Number != 2 {
		t.Errorf("notes = %+v", notes)
	}

	// Without a summary the Overview falls back to the agent's last output.
	det := h.detail(sess.ID)
	if det.Summary != nil {
		t.Error("summary before report_progress")
	}
	h.ev.waitScreen(t, sess.AgentPane, "helper-agent-ready")
	deadline := time.Now().Add(waitFor)
	for !strings.Contains(det.LastMessage, "helper-agent-ready") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		det = h.detail(sess.ID)
	}
	if !strings.Contains(det.LastMessage, "helper-agent-ready") {
		t.Errorf("last message fallback = %q", det.LastMessage)
	}

	res = h.mcp(sess.ID, protocol.ToolReportProgress, protocol.ReportProgressArgs{
		Done: []string{"Added token cache"}, FilesChanged: []string{"src\\auth\\session.go"},
		Decisions: []string{"TTL 5m"}, OpenQuestions: []string{"per-user?"}, Next: "Update callers",
	})
	if res.IsError {
		t.Fatalf("report_progress = %+v", res)
	}
	sum := decode[core.Summary](t, h.ev.wait(t, protocol.TopicSummary, nil))
	if sum.SessionID != sess.ID || sum.Done[0] != "Added token cache" || sum.FilesChanged[0] != "src/auth/session.go" || sum.Next != "Update callers" {
		t.Errorf("summary.updated = %+v", sum)
	}
	var got core.Summary
	h.mustCall(protocol.MethodSummaryGet, protocol.SessionRef{SessionID: sess.ID}, &got)
	if got.Next != "Update callers" {
		t.Errorf("summary.get = %+v", got)
	}
	det = h.detail(sess.ID)
	if det.Summary == nil || det.LastMessage != "" {
		t.Errorf("detail with a summary: %+v / %q", det.Summary, det.LastMessage)
	}
	if bad := h.mcp(sess.ID, protocol.ToolReportProgress, protocol.ReportProgressArgs{}); !bad.IsError {
		t.Error("empty report accepted")
	}
	// Unknown sessions and tools.
	if r := h.mcp("ses_nope", protocol.ToolReportProgress, protocol.ReportProgressArgs{Done: []string{"x"}}); !r.IsError {
		t.Error("unknown session accepted")
	}
	if err := h.call(protocol.MethodMCP, protocol.MCPParams{SessionID: sess.ID, Tool: "approve_everything"}, nil); err == nil {
		t.Error("unknown tool accepted: there must be no approval tool")
	}
}

func agentPrompt() agent.HookEvent      { return agent.HookEvent{Kind: agent.HookPromptSubmit} }
func agentPromptEvent() agent.HookEvent { return agentPrompt() }
func agentStop() agent.HookEvent        { return agent.HookEvent{Kind: agent.HookStop} }

const specRequirements = `# Token refresh

### Requirement 1: Refresh

**Acceptance criteria**
1. WHEN a token expires within 60 seconds, THE SYSTEM SHALL refresh it.
2. IF refresh fails, THEN THE SYSTEM SHALL return an error.

### Requirement 2: Logging

1. THE SYSTEM SHALL log refreshes.
`

const specTasks = `# Tasks

- [ ] 1. Add cache
  - _Requirements: R1.1_
- [ ] 2. Update callers
  - [ ] 2.1 api package
`

func TestMCPSpecTools(t *testing.T) {
	h := newHarness(t)
	// The spec exists only in the main checkout (not committed), as when the
	// user drafts it before starting sessions.
	specDir := filepath.Join(h.repo, ".multiplex", "specs", "auth-refresh")
	writeFile(t, filepath.Join(specDir, "requirements.md"), specRequirements)
	writeFile(t, filepath.Join(specDir, "design.md"), "# Design\nUse a cache.\n")
	writeFile(t, filepath.Join(specDir, "tasks.md"), specTasks)
	sess := h.newSession("helper", "specs")

	var infos []protocol.SpecInfo
	h.mustCall(protocol.MethodSpecList, protocol.WorkspaceRef{WorkspaceID: h.ws.ID}, &infos)
	if len(infos) != 1 || infos[0].Slug != "auth-refresh" || infos[0].TasksTotal != 3 || infos[0].CriteriaTotal != 3 {
		t.Errorf("spec.list = %+v", infos)
	}
	var doc protocol.SpecDoc
	h.mustCall(protocol.MethodSpecGet, protocol.SpecGetParams{WorkspaceID: h.ws.ID, Slug: "auth-refresh"}, &doc)
	if doc.Design == "" || doc.Tasks != specTasks {
		t.Errorf("spec.get = %+v", doc)
	}
	if err := h.call(protocol.MethodSpecGet, protocol.SpecGetParams{WorkspaceID: h.ws.ID, Slug: "../x"}, nil); err == nil {
		t.Error("bad slug accepted")
	}

	res := h.mcp(sess.ID, protocol.ToolSpecList, protocol.SpecListArgs{})
	if res.IsError || !strings.Contains(res.Text, "auth-refresh") {
		t.Errorf("spec_list = %+v", res)
	}
	res = h.mcp(sess.ID, protocol.ToolSpecGet, protocol.SpecGetArgs{Slug: "auth-refresh", Requirement: "R1.2"})
	if res.IsError || !strings.Contains(res.Text, "IF refresh fails") {
		t.Errorf("spec_get R1.2 = %+v", res)
	}
	res = h.mcp(sess.ID, protocol.ToolSpecGet, protocol.SpecGetArgs{Slug: "auth-refresh", Requirement: "R2"})
	if res.IsError || !strings.Contains(res.Text, "log refreshes") || strings.Contains(res.Text, "Requirement 1") {
		t.Errorf("spec_get R2 = %+v", res)
	}
	res = h.mcp(sess.ID, protocol.ToolSpecGet, protocol.SpecGetArgs{Slug: "auth-refresh", File: "tasks"})
	if res.Text != specTasks {
		t.Errorf("spec_get tasks = %q", res.Text)
	}
	if res := h.mcp(sess.ID, protocol.ToolSpecGet, protocol.SpecGetArgs{Slug: "missing"}); !res.IsError {
		t.Error("missing spec accepted")
	}

	// task_update rewrites tasks.md in the WORKTREE, not the main checkout.
	res = h.mcp(sess.ID, protocol.ToolTaskUpdate, protocol.TaskUpdateArgs{Slug: "auth-refresh", TaskID: "1", Status: "done", Note: "cache added"})
	if res.IsError {
		t.Fatalf("task_update = %+v", res)
	}
	wtTasks, err := os.ReadFile(filepath.Join(sess.WorktreePath, ".multiplex", "specs", "auth-refresh", "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wtTasks), "- [x] 1. Add cache") || !strings.Contains(string(wtTasks), "  - Note: cache added") {
		t.Errorf("worktree tasks.md:\n%s", wtTasks)
	}
	mainTasks, _ := os.ReadFile(filepath.Join(specDir, "tasks.md"))
	if string(mainTasks) != specTasks {
		t.Error("task_update changed the main checkout")
	}
	// A second update builds on the worktree copy.
	h.mcp(sess.ID, protocol.ToolTaskUpdate, protocol.TaskUpdateArgs{Slug: "auth-refresh", TaskID: "2.1", Status: "in-progress"})
	wtTasks, _ = os.ReadFile(filepath.Join(sess.WorktreePath, ".multiplex", "specs", "auth-refresh", "tasks.md"))
	if !strings.Contains(string(wtTasks), "[x] 1. Add cache") || !strings.Contains(string(wtTasks), "[~] 2.1 api package") {
		t.Errorf("second update lost the first:\n%s", wtTasks)
	}
	if res := h.mcp(sess.ID, protocol.ToolTaskUpdate, protocol.TaskUpdateArgs{Slug: "auth-refresh", TaskID: "2", Status: "blocked"}); !res.IsError {
		t.Error("blocked without a note accepted")
	}
	if res := h.mcp(sess.ID, protocol.ToolTaskUpdate, protocol.TaskUpdateArgs{Slug: "auth-refresh", TaskID: "9", Status: "done"}); !res.IsError {
		t.Error("missing task accepted")
	}
	// The change shows up in the session's diff.
	var d protocol.DiffResult
	h.mustCall(protocol.MethodDiffGet, protocol.SessionRef{SessionID: sess.ID}, &d)
	if !strings.Contains(d.Raw, "tasks.md") {
		t.Errorf("task update is not in the diff:\n%s", d.Raw)
	}

	res = h.mcp(sess.ID, protocol.ToolCriterionVerify, protocol.CriterionVerifyArgs{Slug: "auth-refresh", Criterion: "R1.1", Evidence: "TestRefresh in cache_test.go:10-30"})
	if res.IsError {
		t.Fatalf("criterion_verify = %+v", res)
	}
	h.mcp(sess.ID, protocol.ToolCriterionVerify, protocol.CriterionVerifyArgs{Slug: "auth-refresh", Criterion: "R1.1", Evidence: "updated evidence"})
	h.mcp(sess.ID, protocol.ToolCriterionVerify, protocol.CriterionVerifyArgs{Slug: "auth-refresh", Criterion: "R1.2", Evidence: "TestRefreshFails"})
	ver, _ := os.ReadFile(filepath.Join(sess.WorktreePath, ".multiplex", "specs", "auth-refresh", "verification.md"))
	if strings.Count(string(ver), "R1.1:") != 1 || !strings.Contains(string(ver), "updated evidence") || !strings.Contains(string(ver), "R1.2: TestRefreshFails") {
		t.Errorf("verification.md:\n%s", ver)
	}
	if res := h.mcp(sess.ID, protocol.ToolCriterionVerify, protocol.CriterionVerifyArgs{Slug: "auth-refresh", Criterion: "R9.9", Evidence: "x"}); !res.IsError {
		t.Error("unknown criterion accepted")
	}
	if res := h.mcp(sess.ID, protocol.ToolCriterionVerify, protocol.CriterionVerifyArgs{Slug: "auth-refresh", Criterion: "R1.1\n- R9.9", Evidence: "x"}); !res.IsError {
		t.Error("injected criterion accepted")
	}
}

func TestMCPOrchestration(t *testing.T) {
	h := newHarness(t)
	orch := h.newSession("helper", "orchestrator")

	res := h.mcp(orch.ID, protocol.ToolSessionSpawn, protocol.SessionSpawnArgs{Task: "implement the cache", Name: "cache impl", Role: "implementer", Agent: "helper"})
	if res.IsError {
		t.Fatalf("session_spawn = %+v", res)
	}
	var spawned struct {
		SessionID core.SessionID `json:"session_id"`
		Slug      string         `json:"slug"`
		Branch    string         `json:"branch"`
	}
	if err := json.Unmarshal([]byte(res.Text), &spawned); err != nil || spawned.SessionID == "" {
		t.Fatalf("spawn result %q: %v", res.Text, err)
	}
	if spawned.Slug != "cache-impl" || spawned.Branch != "mpx/cache-impl" {
		t.Errorf("spawned = %+v", spawned)
	}
	var sessions []core.Session
	h.mustCall(protocol.MethodSessionList, protocol.SessionListParams{WorkspaceID: h.ws.ID}, &sessions)
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(sessions))
	}
	if r := h.mcp(orch.ID, protocol.ToolSessionSpawn, protocol.SessionSpawnArgs{Task: "x", Role: "no-such-role"}); !r.IsError {
		t.Error("unknown role accepted")
	}

	// session_wait blocks until the child settles.
	waitDone := make(chan protocol.MCPResult, 1)
	go func() {
		waitDone <- h.mcp(orch.ID, protocol.ToolSessionWait, protocol.SessionWaitArgs{Sessions: []string{"cache-impl"}, TimeoutSeconds: 60})
	}()
	select {
	case r := <-waitDone:
		t.Fatalf("session_wait returned while the child was starting: %+v", r)
	case <-time.After(300 * time.Millisecond):
	}
	h.mcp(spawned.SessionID, protocol.ToolNoteToUser, protocol.NoteToUserArgs{Title: "child note"})
	h.mcp(spawned.SessionID, protocol.ToolReportProgress, protocol.ReportProgressArgs{Done: []string{"child done"}})
	h.hook(spawned.SessionID, agentStop())
	select {
	case r := <-waitDone:
		var out struct {
			TimedOut bool `json:"timed_out"`
			Sessions []struct {
				Slug    string        `json:"slug"`
				State   string        `json:"state"`
				Summary *core.Summary `json:"summary"`
				Notes   []struct {
					Title string `json:"title"`
				} `json:"notes"`
			} `json:"sessions"`
		}
		if err := json.Unmarshal([]byte(r.Text), &out); err != nil || r.IsError {
			t.Fatalf("wait result %+v: %v", r, err)
		}
		if out.TimedOut || len(out.Sessions) != 1 || out.Sessions[0].State != "done" ||
			out.Sessions[0].Summary == nil || out.Sessions[0].Summary.Done[0] != "child done" ||
			len(out.Sessions[0].Notes) != 1 || out.Sessions[0].Notes[0].Title != "child note" {
			t.Errorf("wait result = %+v", out)
		}
	case <-time.After(waitFor):
		t.Fatal("session_wait did not return after the child finished")
	}

	// Timeout.
	h.hook(spawned.SessionID, agentPromptEvent())
	r := h.mcp(orch.ID, protocol.ToolSessionWait, protocol.SessionWaitArgs{Sessions: []string{string(spawned.SessionID)}, TimeoutSeconds: 1})
	if !strings.Contains(r.Text, `"timed_out": true`) {
		t.Errorf("wait should time out: %+v", r)
	}

	r = h.mcp(orch.ID, protocol.ToolSessionStatus, protocol.SessionStatusArgs{})
	if !strings.Contains(r.Text, "cache-impl") || strings.Contains(r.Text, `"slug": "orchestrator"`) {
		t.Errorf("session_status lists the caller or misses the child: %s", r.Text)
	}
	if r := h.mcp(orch.ID, protocol.ToolSessionStatus, protocol.SessionStatusArgs{Sessions: []string{"nope"}}); !r.IsError {
		t.Error("unknown session accepted")
	}

	// session_message types into the other agent's pane.
	r = h.mcp(orch.ID, protocol.ToolSessionMessage, protocol.SessionMessageArgs{Session: "cache-impl", Text: "also add a TTL"})
	if r.IsError {
		t.Fatalf("session_message = %+v", r)
	}
	var child core.Session
	for _, s := range h.detailSessions() {
		if s.Slug == "cache-impl" {
			child = s
		}
	}
	h.ev.waitScreen(t, child.AgentPane, "Message from session orchestrator: also add a TTL")
	if r := h.mcp(orch.ID, protocol.ToolSessionMessage, protocol.SessionMessageArgs{Session: "orchestrator", Text: "hi"}); !r.IsError {
		t.Error("a session messaged itself")
	}
}

func (h *harness) detailSessions() []core.Session {
	var out []core.Session
	h.mustCall(protocol.MethodSessionList, protocol.SessionListParams{}, &out)
	return out
}
