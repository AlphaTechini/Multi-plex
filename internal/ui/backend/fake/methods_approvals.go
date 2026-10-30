package fake

import (
	"context"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/store"
)

// PendingApprovals lists unresolved approvals.
func (b *Backend) PendingApprovals(ctx context.Context) ([]core.ApprovalRequest, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("PendingApprovals")
	return append([]core.ApprovalRequest(nil), b.approvals...), ctxErr(ctx)
}

// RespondApproval resolves a pending approval. With the typed-code method an
// approval needs the right Code; a wrong one is refused with an error and the
// request stays pending.
func (b *Backend) RespondApproval(ctx context.Context, p protocol.ApprovalRespondParams) (core.ApprovalDecision, error) {
	if b.delay > 0 && p.Outcome == core.Approved {
		select {
		case <-time.After(b.delay):
		case <-ctx.Done():
			return core.ApprovalDecision{}, ctx.Err()
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("RespondApproval")
	idx := -1
	for i, a := range b.approvals {
		if a.ID == p.RequestID {
			idx = i
		}
	}
	if idx < 0 {
		return core.ApprovalDecision{}, notFound("approval")
	}
	if p.Outcome == core.Approved && b.method == "typed-code" && p.Code != b.code {
		return core.ApprovalDecision{}, &protocol.Error{Code: protocol.CodeDenied, Message: "wrong code"}
	}
	req := b.approvals[idx]
	b.approvals = append(b.approvals[:idx], b.approvals[idx+1:]...)
	d := core.ApprovalDecision{RequestID: req.ID, Outcome: p.Outcome, Message: p.Message, Method: b.method, At: b.now()}
	b.audit = append(b.audit, store.AuditEntry{Request: req, Decision: d})
	b.emit(protocol.TopicApprovalDone, d)

	if s := b.session(req.SessionID); s != nil {
		if s.State == core.StateBlocked && !b.hasPendingLocked(s.ID) {
			s.State, s.StateDetail, s.UpdatedAt = core.StateWorking, "", b.now()
			b.emit(protocol.TopicSession, *s)
		}
		if pane := b.panes[s.AgentPane]; pane != nil {
			if p.Outcome == core.Approved {
				pane.entries = append(pane.entries, entry{kind: entTool, name: "Bash", arg: "Remove-Item testdata/golden", result: "Removed 14 files"})
				pane.working, pane.startedT = true, b.now()
			} else {
				msg := "Understood. I will not delete it and will regenerate the files in place instead."
				if p.Message != "" {
					msg = "Understood: " + p.Message
				}
				pane.entries = append(pane.entries, entry{kind: entSay, text: msg})
			}
			b.pushFrameLocked(pane, false)
		}
	}
	return d, ctxErr(ctx)
}

func (b *Backend) hasPendingLocked(id core.SessionID) bool {
	for _, a := range b.approvals {
		if a.SessionID == id {
			return true
		}
	}
	return false
}

// ApprovalAudit returns the approvals decided for a session.
func (b *Backend) ApprovalAudit(ctx context.Context, id core.SessionID) ([]store.AuditEntry, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("ApprovalAudit")
	var out []store.AuditEntry
	for _, e := range b.audit {
		if e.Request.SessionID == id {
			out = append(out, e)
		}
	}
	return out, ctxErr(ctx)
}

// Checkpoints lists fake checkpoints.
func (b *Backend) Checkpoints(ctx context.Context, id core.SessionID) ([]core.Checkpoint, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Checkpoints")
	now := b.now()
	return []core.Checkpoint{
		{SessionID: id, N: 1, Ref: "refs/multiplex/checkpoints/" + string(id) + "/1", Commit: "5d1a9c2e", Reason: "before recursive delete", At: now.Add(-20 * time.Minute)},
		{SessionID: id, N: 2, Ref: "refs/multiplex/checkpoints/" + string(id) + "/2", Commit: "8e07b34a", Reason: "start of turn 5", At: now.Add(-6 * time.Minute)},
	}, ctxErr(ctx)
}

// RestoreCheckpoint records the restore (nothing to roll back in memory).
func (b *Backend) RestoreCheckpoint(ctx context.Context, id core.SessionID, n int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("RestoreCheckpoint")
	return ctxErr(ctx)
}
