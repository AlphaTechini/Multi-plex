package memory

import (
	"cmp"
	"context"
	"slices"

	"github.com/alphatechini/multi-plex/internal/core"
)

// PutComment inserts or replaces a comment by ID.
func (s *Store) PutComment(_ context.Context, c core.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.comments[c.ID] = cloneComment(c)
	return nil
}

// ListComments returns a session's comments ordered by Path, then NewStart,
// then ID.
func (s *Store) ListComments(_ context.Context, id core.SessionID) ([]core.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []core.Comment
	for _, c := range s.comments {
		if c.SessionID == id {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b core.Comment) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.NewStart, b.NewStart), cmp.Compare(a.ID, b.ID))
	})
	return out, nil
}

// DeleteComment removes a comment. Deleting a missing comment is not an error.
func (s *Store) DeleteComment(_ context.Context, commentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.comments, commentID)
	return nil
}
