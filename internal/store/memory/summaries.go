package memory

import (
	"context"
	"slices"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

// AddSummary appends a summary to the session's history.
func (s *Store) AddSummary(_ context.Context, sum core.Summary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaries[sum.SessionID] = append(s.summaries[sum.SessionID], cloneSummary(sum))
	return nil
}

// LatestSummary returns the newest summary of a session or store.ErrNotFound.
func (s *Store) LatestSummary(ctx context.Context, id core.SessionID) (core.Summary, error) {
	got, _ := s.ListSummaries(ctx, id, 1)
	if len(got) == 0 {
		return core.Summary{}, store.ErrNotFound
	}
	return got[0], nil
}

// ListSummaries returns a session's summaries newest first (by At, later
// insertions first on ties). A limit <= 0 means no limit.
func (s *Store) ListSummaries(_ context.Context, id core.SessionID, limit int) ([]core.Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.summaries[id]
	out := make([]core.Summary, 0, len(src))
	for i := len(src) - 1; i >= 0; i-- {
		out = append(out, cloneSummary(src[i]))
	}
	slices.SortStableFunc(out, func(a, b core.Summary) int { return b.At.Compare(a.At) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
