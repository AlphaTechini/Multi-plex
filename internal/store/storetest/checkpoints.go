package storetest

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

func testCheckpoints(t *testing.T, s store.Store) {
	if got, err := s.ListCheckpoints(ctx, "s1"); err != nil || len(got) != 0 {
		t.Fatalf("ListCheckpoints on empty store = %v, %v", got, err)
	}
	c3, c1, c2 := fullCheckpoint("s1", 3), fullCheckpoint("s1", 1), fullCheckpoint("s1", 2)
	other := fullCheckpoint("s2", 1)
	for _, c := range []core.Checkpoint{c3, c1, c2, other} {
		must(t, "AddCheckpoint", s.AddCheckpoint(ctx, c))
	}
	list, err := s.ListCheckpoints(ctx, "s1")
	must(t, "ListCheckpoints", err)
	if len(list) != 3 {
		t.Fatalf("ListCheckpoints len = %d, want 3", len(list))
	}
	for i, want := range []core.Checkpoint{c1, c2, c3} {
		same(t, "ListCheckpoints order", normCheckpoint(list[i]), normCheckpoint(want))
		sameTime(t, "checkpoint At", list[i].At, want.At)
	}

	// The same (session, N) replaces rather than duplicates.
	c2.Commit, c2.Reason = "fedcba", "replaced"
	must(t, "AddCheckpoint replace", s.AddCheckpoint(ctx, c2))
	list, _ = s.ListCheckpoints(ctx, "s1")
	if len(list) != 3 || list[1].Commit != "fedcba" || list[1].Reason != "replaced" {
		t.Errorf("after replace: %+v", list)
	}
}
