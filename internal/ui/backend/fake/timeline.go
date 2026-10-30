package fake

import (
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// runTimeline plays the scripted demo: every interval one step runs.
func (b *Backend) runTimeline(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for n := 1; ; n++ {
		select {
		case <-b.done:
			return
		case <-t.C:
			b.step(n)
		}
	}
}

// step runs scripted step n (1-based). Steps are small: a few tool lines in
// the agent terminal, a new card, a summary update, state changes. Between
// them the working agent's spinner and elapsed time keep the terminal alive.
func (b *Backend) step(n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	select {
	case <-b.done:
		return
	default:
	}
	auth := b.session(SessionAuth)
	if auth == nil {
		return
	}
	p := b.panes[auth.AgentPane]
	now := b.now()
	switch n {
	case 2:
		p.entries = append(p.entries, entry{kind: entTool, name: "Read", arg: "src/api/client.go", result: "Read 214 lines"})
	case 3:
		p.entries = append(p.entries, entry{kind: entTool, name: "Update", arg: "src/worker/refresh.go", result: "Updated with 5 additions and 5 removals"})
		b.emit(protocol.TopicEdit, core.EditEvent{SessionID: SessionAuth, Path: "src/worker/refresh.go", StartLine: 17, EndLine: 28, At: now})
	case 4:
		b.addNoteLocked(SessionAuth, core.SeverityInfo, "Added a race test for the cache",
			"Eight goroutines ask for the same user at once; the source must be called at most twice.",
			"src/auth/session_test.go:L44-L66")
	case 5:
		sum := b.summaries[SessionAuth]
		sum.Done = append(sum.Done, "Updated Client.Do to use the cached token")
		sum.At = now
		b.emit(protocol.TopicSummary, *sum)
		auth.UpdatedAt = now
		b.emit(protocol.TopicSession, *auth)
	case 6:
		p.entries = append(p.entries, entry{kind: entTool, name: "Bash", arg: "go test -race ./internal/auth/...", result: "ok   internal/auth   1.208s"})
		b.emit(protocol.TopicDiffChanged, protocol.SessionRef{SessionID: SessionAuth})
	case 8:
		b.emit(protocol.TopicToast, protocol.Toast{Level: "info", Text: "Checkpoint 3 created for auth-refactor"})
	case 10:
		p.entries = append(p.entries, entry{kind: entSay, text: "The worker now uses the shared cache. Waiting on your answer about per-user versus global before I touch the remaining callers."})
	case 14:
		// A second approval, for the session on screen: the modal opens by itself.
		req := core.ApprovalRequest{
			ID: b.nextID("apr"), SessionID: SessionAuth, Kind: core.ApprovalCommand, Tool: "Bash",
			Command: "git push --force origin mpx/auth-refactor", Cwd: auth.WorktreePath,
			Layer: "Layer 5 pushes need approval", Rule: "push · .multiplex/rules.toml:3", At: now,
		}
		b.approvals = append(b.approvals, req)
		auth.State, auth.StateDetail, auth.UpdatedAt = core.StateBlocked, "needs approval", now
		p.working = false
		b.emit(protocol.TopicApprovalPending, b.pendingEvent(req))
		b.emit(protocol.TopicSession, *auth)
	}
	// Alive: the working agents' spinners and elapsed counters.
	for _, s := range b.sessions {
		if pane := b.panes[s.AgentPane]; pane != nil && pane.working && !pane.exited {
			pane.spin++
			b.pushFrameLocked(pane, false)
		}
	}
}
