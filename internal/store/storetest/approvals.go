package storetest

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

func testApprovals(t *testing.T, s store.Store) {
	if got, err := s.ListApprovals(ctx, "s1", 10); err != nil || len(got) != 0 {
		t.Fatalf("ListApprovals on empty store = %v, %v", got, err)
	}

	// Logged out of time order; listing is newest first by decision time.
	var entries [4]store.AuditEntry
	for i, at := range []int{10, 40, 20, 30} {
		r, d := fullApproval(string(rune('a'+i)), "s1", at)
		if i == 2 {
			r.Paths, d.Outcome, d.Message = nil, core.Approved, ""
		}
		entries[i] = store.AuditEntry{Request: r, Decision: d}
		must(t, "LogApproval", s.LogApproval(ctx, r, d))
	}
	r, d := fullApproval("z", "s2", 99)
	must(t, "LogApproval other", s.LogApproval(ctx, r, d))

	all, err := s.ListApprovals(ctx, "s1", 0)
	must(t, "ListApprovals", err)
	if len(all) != 4 {
		t.Fatalf("ListApprovals len = %d, want 4", len(all))
	}
	for i, want := range []store.AuditEntry{entries[1], entries[3], entries[2], entries[0]} {
		same(t, "ListApprovals order", normAudit(all[i]), normAudit(want))
		sameTime(t, "request At", all[i].Request.At, want.Request.At)
		sameTime(t, "decision At", all[i].Decision.At, want.Decision.At)
	}

	two, err := s.ListApprovals(ctx, "s1", 2)
	must(t, "ListApprovals limit", err)
	if len(two) != 2 || two[0].Request.ID != "b" || two[1].Request.ID != "d" {
		t.Errorf("ListApprovals(limit 2) = %+v", two)
	}

	got, err := s.ListApprovals(ctx, "s2", 5)
	must(t, "ListApprovals s2", err)
	if len(got) != 1 || got[0].Request.ID != "z" {
		t.Errorf("ListApprovals s2 = %+v", got)
	}
}
