package storetest

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

// populate fills every per-session table for sid.
func populate(t *testing.T, s store.Store, sid string) {
	t.Helper()
	must(t, "PutSession", s.PutSession(ctx, fullSession(sid, "w", 1)))
	must(t, "AddSummary", s.AddSummary(ctx, fullSummary(sid, 1)))
	_, err := s.AddNote(ctx, fullNote(sid))
	must(t, "AddNote", err)
	must(t, "PutComment", s.PutComment(ctx, fullComment("c-"+sid, sid, "f.go", 1, 1)))
	must(t, "AddCheckpoint", s.AddCheckpoint(ctx, fullCheckpoint(sid, 1)))
	r, d := fullApproval("ap-"+sid, sid, 1)
	must(t, "LogApproval", s.LogApproval(ctx, r, d))
}

func testDeleteSession(t *testing.T, s store.Store) {
	populate(t, s, "gone")
	populate(t, s, "kept")
	must(t, "DeleteSession", s.DeleteSession(ctx, "gone"))

	_, err := s.GetSession(ctx, "gone")
	notFound(t, "GetSession", err)
	_, err = s.LatestSummary(ctx, "gone")
	notFound(t, "LatestSummary", err)
	counts := func(sid core.SessionID) (sum, notes, comments, cps, approvals int) {
		a, _ := s.ListSummaries(ctx, sid, 0)
		b, _ := s.ListNotes(ctx, sid)
		c, _ := s.ListComments(ctx, sid)
		d, _ := s.ListCheckpoints(ctx, sid)
		e, _ := s.ListApprovals(ctx, sid, 0)
		return len(a), len(b), len(c), len(d), len(e)
	}
	if sum, notes, comments, cps, _ := counts("gone"); sum+notes+comments+cps != 0 {
		t.Errorf("rows left after delete: summaries=%d notes=%d comments=%d checkpoints=%d", sum, notes, comments, cps)
	}
	// The audit log is append-only: it survives the session.
	if _, _, _, _, ap := counts("gone"); ap != 1 {
		t.Errorf("approvals after delete = %d, want 1", ap)
	}
	if sum, notes, comments, cps, ap := counts("kept"); sum != 1 || notes != 1 || comments != 1 || cps != 1 || ap != 1 {
		t.Errorf("other session affected: %d %d %d %d %d", sum, notes, comments, cps, ap)
	}
	if _, err := s.GetSession(ctx, "kept"); err != nil {
		t.Errorf("GetSession kept: %v", err)
	}

	// Numbering restarts for a recreated session.
	n, err := s.AddNote(ctx, fullNote("gone"))
	must(t, "AddNote after delete", err)
	if n.Number != 1 {
		t.Errorf("Number after delete = %d, want 1", n.Number)
	}
}
