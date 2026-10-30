package storetest

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

func testComments(t *testing.T, s store.Store) {
	if got, err := s.ListComments(ctx, "s1"); err != nil || len(got) != 0 {
		t.Fatalf("ListComments on empty store = %v, %v", got, err)
	}
	must(t, "DeleteComment missing", s.DeleteComment(ctx, "missing"))

	// Insert out of order; List orders by path, then NewStart.
	c1 := fullComment("c1", "s1", "b/file.go", 5, 1)
	c2 := fullComment("c2", "s1", "a/file.go", 50, 2)
	c3 := fullComment("c3", "s1", "a/file.go", 7, 3)
	c4 := fullComment("c4", "s1", "b/file.go", 1, 4)
	other := fullComment("c5", "s2", "a/file.go", 1, 5)
	for _, c := range []core.Comment{c1, c2, c3, c4, other} {
		must(t, "PutComment", s.PutComment(ctx, c))
	}

	list, err := s.ListComments(ctx, "s1")
	must(t, "ListComments", err)
	if len(list) != 4 {
		t.Fatalf("ListComments len = %d, want 4", len(list))
	}
	for i, want := range []core.Comment{c3, c2, c4, c1} {
		same(t, "ListComments order", normComment(list[i]), normComment(want))
		sameTime(t, "comment At", list[i].At, want.At)
	}

	// Upsert by ID: all fields replaced, flags can reset.
	c3.Body, c3.Status, c3.Changed, c3.NewStart, c3.OldStart, c3.OldEnd = "edited", core.CommentSent, false, 60, 0, 0
	must(t, "PutComment upsert", s.PutComment(ctx, c3))
	list, _ = s.ListComments(ctx, "s1")
	if len(list) != 4 {
		t.Fatalf("upsert changed comment count to %d", len(list))
	}
	for i, want := range []core.Comment{c2, c3, c4, c1} {
		same(t, "ListComments after upsert", normComment(list[i]), normComment(want))
	}

	must(t, "DeleteComment", s.DeleteComment(ctx, "c2"))
	list, _ = s.ListComments(ctx, "s1")
	if len(list) != 3 || list[0].ID != "c3" {
		t.Errorf("after delete: %+v", list)
	}
	if got, _ := s.ListComments(ctx, "s2"); len(got) != 1 {
		t.Errorf("other session comments = %d, want 1", len(got))
	}
}
