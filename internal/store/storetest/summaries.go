package storetest

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

func testSummaries(t *testing.T, s store.Store) {
	_, err := s.LatestSummary(ctx, "s1")
	notFound(t, "LatestSummary empty", err)
	if got, err := s.ListSummaries(ctx, "s1", 0); err != nil || len(got) != 0 {
		t.Fatalf("ListSummaries on empty store = %v, %v", got, err)
	}

	// Insert out of time order: newest is the one with the largest At.
	sums := []core.Summary{fullSummary("s1", 20), fullSummary("s1", 40), fullSummary("s1", 10), fullSummary("s1", 30)}
	for i := range sums {
		sums[i].Next = string(rune('A' + i))
		must(t, "AddSummary", s.AddSummary(ctx, sums[i]))
	}
	other := fullSummary("s2", 99)
	must(t, "AddSummary other", s.AddSummary(ctx, other))

	latest, err := s.LatestSummary(ctx, "s1")
	must(t, "LatestSummary", err)
	same(t, "LatestSummary", normSummary(latest), normSummary(sums[1]))
	sameTime(t, "LatestSummary.At", latest.At, sums[1].At)

	all, err := s.ListSummaries(ctx, "s1", 0)
	must(t, "ListSummaries", err)
	if len(all) != 4 {
		t.Fatalf("ListSummaries len = %d, want 4", len(all))
	}
	for i, want := range []core.Summary{sums[1], sums[3], sums[0], sums[2]} {
		same(t, "ListSummaries order", normSummary(all[i]), normSummary(want))
	}

	two, err := s.ListSummaries(ctx, "s1", 2)
	must(t, "ListSummaries limit", err)
	if len(two) != 2 || two[0].Next != sums[1].Next || two[1].Next != sums[3].Next {
		t.Errorf("ListSummaries(limit 2) = %+v", two)
	}

	got, err := s.LatestSummary(ctx, "s2")
	must(t, "LatestSummary s2", err)
	same(t, "LatestSummary s2", normSummary(got), normSummary(other))
}
