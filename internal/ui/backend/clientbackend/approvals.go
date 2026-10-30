package clientbackend

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/store"
)

// PendingApprovals lists the approvals waiting for a human.
func (b *Backend) PendingApprovals(ctx context.Context) ([]core.ApprovalRequest, error) {
	var out []core.ApprovalRequest
	err := b.call(ctx, protocol.MethodApprovalList, nil, &out)
	return out, err
}

// RespondApproval answers a pending approval. For human-present methods the
// call returns after the OS check finishes, which can take many seconds.
func (b *Backend) RespondApproval(ctx context.Context, p protocol.ApprovalRespondParams) (core.ApprovalDecision, error) {
	var out core.ApprovalDecision
	err := b.call(ctx, protocol.MethodApprovalRespond, p, &out)
	return out, err
}

// ApprovalAudit returns the session's approval audit log.
func (b *Backend) ApprovalAudit(ctx context.Context, id core.SessionID) ([]store.AuditEntry, error) {
	var out []store.AuditEntry
	err := b.call(ctx, protocol.MethodApprovalAudit, sref(id), &out)
	return out, err
}

// Checkpoints lists the session's checkpoints.
func (b *Backend) Checkpoints(ctx context.Context, id core.SessionID) ([]core.Checkpoint, error) {
	var out []core.Checkpoint
	err := b.call(ctx, protocol.MethodCheckpointList, sref(id), &out)
	return out, err
}

// RestoreCheckpoint rolls the session back to checkpoint n.
func (b *Backend) RestoreCheckpoint(ctx context.Context, id core.SessionID, n int) error {
	return b.call(ctx, protocol.MethodCheckpointRestore, protocol.CheckpointRestoreParams{SessionID: id, N: n}, nil)
}
