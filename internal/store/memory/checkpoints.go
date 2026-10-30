package memory

import (
	"context"
	"slices"

	"github.com/alphatechini/multi-plex/internal/core"
)

// AddCheckpoint stores a checkpoint; one with the same session and N is
// replaced.
func (s *Store) AddCheckpoint(_ context.Context, c core.Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.checkpoints[c.SessionID]
	c = cloneCheckpoint(c)
	for i := range list {
		if list[i].N == c.N {
			list[i] = c
			return nil
		}
	}
	s.checkpoints[c.SessionID] = append(list, c)
	return nil
}

// ListCheckpoints returns a session's checkpoints ordered by N ascending.
func (s *Store) ListCheckpoints(_ context.Context, id core.SessionID) ([]core.Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := slices.Clone(s.checkpoints[id])
	slices.SortFunc(out, func(a, b core.Checkpoint) int { return a.N - b.N })
	return out, nil
}
