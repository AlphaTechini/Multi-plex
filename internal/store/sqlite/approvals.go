package sqlite

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

const approvalCols = `request_id, session_id, kind, tool, command, cwd, paths, rule, layer, requested_at,
decision_request_id, outcome, message, method, decided_at`

func scanApproval(r rowScanner) (store.AuditEntry, error) {
	var e store.AuditEntry
	var paths string
	var requested, decided int64
	err := r.Scan(&e.Request.ID, &e.Request.SessionID, &e.Request.Kind, &e.Request.Tool, &e.Request.Command,
		&e.Request.Cwd, &paths, &e.Request.Rule, &e.Request.Layer, &requested,
		&e.Decision.RequestID, &e.Decision.Outcome, &e.Decision.Message, &e.Decision.Method, &decided)
	if err != nil {
		return store.AuditEntry{}, err
	}
	if e.Request.Paths, err = decodeStrings(paths); err != nil {
		return store.AuditEntry{}, err
	}
	e.Request.At, e.Decision.At = fromNanos(requested), fromNanos(decided)
	return e, nil
}

// LogApproval appends a request and its decision to the audit log. There is
// deliberately no way to change or remove entries: the table also rejects
// UPDATE and DELETE with triggers.
func (s *Store) LogApproval(ctx context.Context, r core.ApprovalRequest, d core.ApprovalDecision) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO approvals (`+approvalCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.SessionID, r.Kind, r.Tool, r.Command, r.Cwd, encodeStrings(r.Paths), r.Rule, r.Layer, nanos(r.At),
		d.RequestID, d.Outcome, d.Message, d.Method, nanos(d.At))
	if err != nil {
		return fmt.Errorf("sqlite: log approval: %w", err)
	}
	return nil
}

// ListApprovals returns a session's audit entries newest first (by decision
// time, later insertions first on ties). A limit <= 0 means no limit.
func (s *Store) ListApprovals(ctx context.Context, id core.SessionID, limit int) ([]store.AuditEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+approvalCols+` FROM approvals WHERE session_id = ? ORDER BY decided_at DESC, seq DESC LIMIT ?`,
		id, limitArg(limit))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list approvals: %w", err)
	}
	return collect(rows, scanApproval, "approvals")
}
