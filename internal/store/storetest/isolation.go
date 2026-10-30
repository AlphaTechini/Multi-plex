package storetest

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/store"
)

// testIsolation checks that callers cannot reach stored data through the
// slices they pass in or receive.
func testIsolation(t *testing.T, s store.Store) {
	sum := fullSummary("s1", 1)
	want := fullSummary("s1", 1)
	must(t, "AddSummary", s.AddSummary(ctx, sum))
	sum.Done[0] = "mutated after put"

	got, err := s.LatestSummary(ctx, "s1")
	must(t, "LatestSummary", err)
	same(t, "summary after input mutation", normSummary(got), normSummary(want))
	got.Done[0], got.FilesChanged[0] = "mutated after get", "mutated"
	got2, _ := s.LatestSummary(ctx, "s1")
	same(t, "summary after output mutation", normSummary(got2), normSummary(want))

	note := fullNote("s1")
	stored, err := s.AddNote(ctx, note)
	must(t, "AddNote", err)
	note.Refs[0] = "mutated input"
	stored.Refs[1] = "mutated result"
	list, _ := s.ListNotes(ctx, "s1")
	list[0].Refs[0] = "mutated listing"
	again, _ := s.ListNotes(ctx, "s1")
	same(t, "note refs", again[0].Refs, fullNote("s1").Refs)

	r, d := fullApproval("a", "s1", 1)
	must(t, "LogApproval", s.LogApproval(ctx, r, d))
	r.Paths[0] = "mutated input"
	entries, _ := s.ListApprovals(ctx, "s1", 1)
	entries[0].Request.Paths[1] = "mutated listing"
	entries, _ = s.ListApprovals(ctx, "s1", 1)
	wantR, _ := fullApproval("a", "s1", 1)
	same(t, "approval paths", entries[0].Request.Paths, wantR.Paths)
}
