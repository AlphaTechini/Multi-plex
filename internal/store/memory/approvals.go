package memory

import (
	"context"
	"slices"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

// LogApproval appends a request and its decision to the audit log.
func (s *Store) LogApproval(_ context.Context, r core.ApprovalRequest, d core.ApprovalDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvals = append(s.approvals, cloneAudit(store.AuditEntry{Request: r, Decision: d}))
	return nil
}

// ListApprovals returns a session's audit entries newest first (by decision
// time, later insertions first on ties). A limit <= 0 means no limit.
func (s *Store) ListApprovals(_ context.Context, id core.SessionID, limit int) ([]store.AuditEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []store.AuditEntry
	for i := len(s.approvals) - 1; i >= 0; i-- {
		if e := s.approvals[i]; e.Request.SessionID == id {
			out = append(out, cloneAudit(e))
		}
	}
	slices.SortStableFunc(out, func(a, b store.AuditEntry) int { return b.Decision.At.Compare(a.Decision.At) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
