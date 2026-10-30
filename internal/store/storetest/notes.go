package storetest

import (
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/store"
)

func testNotes(t *testing.T, s store.Store) {
	if got, err := s.ListNotes(ctx, "s1"); err != nil || len(got) != 0 {
		t.Fatalf("ListNotes on empty store = %v, %v", got, err)
	}
	notFound(t, "DismissNote missing", s.DismissNote(ctx, "nope"))

	// Numbering is per session, starts at 1, and ignores any Number supplied.
	var ids []string
	for i := 1; i <= 3; i++ {
		n := fullNote("s1")
		n.Number = 99
		got, err := s.AddNote(ctx, n)
		must(t, "AddNote", err)
		if got.Number != i {
			t.Errorf("s1 note %d: Number = %d", i, got.Number)
		}
		if !strings.HasPrefix(got.ID, "note_") {
			t.Errorf("generated ID = %q, want note_ prefix", got.ID)
		}
		ids = append(ids, got.ID)

		want := n
		want.ID, want.Number = got.ID, i
		same(t, "AddNote result", normNote(got), normNote(want))
	}
	if ids[0] == ids[1] || ids[1] == ids[2] || ids[0] == ids[2] {
		t.Errorf("generated IDs not unique: %v", ids)
	}

	// A caller-supplied ID is kept; another session numbers independently.
	n := fullNote("s2")
	n.ID = "my-note"
	got, err := s.AddNote(ctx, n)
	must(t, "AddNote s2", err)
	if got.ID != "my-note" || got.Number != 1 {
		t.Errorf("s2 first note = id %q number %d", got.ID, got.Number)
	}
	got, err = s.AddNote(ctx, fullNote("s1"))
	must(t, "AddNote s1 #4", err)
	if got.Number != 4 {
		t.Errorf("s1 fourth note Number = %d", got.Number)
	}

	list, err := s.ListNotes(ctx, "s1")
	must(t, "ListNotes", err)
	if len(list) != 4 {
		t.Fatalf("ListNotes len = %d, want 4", len(list))
	}
	for i, x := range list {
		if x.Number != i+1 {
			t.Errorf("ListNotes[%d].Number = %d", i, x.Number)
		}
		if x.Dismissed {
			t.Errorf("note %d dismissed by default", x.Number)
		}
		same(t, "note Refs", x.Refs, fullNote("s1").Refs)
		sameTime(t, "note At", x.At, fullNote("s1").At)
	}

	// DismissNote flips only the named note, and is idempotent.
	must(t, "DismissNote", s.DismissNote(ctx, ids[1]))
	must(t, "DismissNote again", s.DismissNote(ctx, ids[1]))
	list, _ = s.ListNotes(ctx, "s1")
	for _, x := range list {
		if want := x.ID == ids[1]; x.Dismissed != want {
			t.Errorf("note %d Dismissed = %v, want %v", x.Number, x.Dismissed, want)
		}
	}
	list, _ = s.ListNotes(ctx, "s2")
	if len(list) != 1 || list[0].Dismissed {
		t.Errorf("s2 notes affected by dismiss: %+v", list)
	}

	// Empty and nil Refs both read back as no refs.
	n = fullNote("s3")
	n.Refs = nil
	got, err = s.AddNote(ctx, n)
	must(t, "AddNote no refs", err)
	if len(got.Refs) != 0 {
		t.Errorf("Refs = %v, want none", got.Refs)
	}
}

// testConcurrentAddNote is the numbering smoke test: 50 goroutines adding to
// one session must get unique consecutive numbers.
func testConcurrentAddNote(t *testing.T, s store.Store) {
	const workers = 50
	type result struct {
		num int
		err error
	}
	results := make(chan result, workers)
	start := make(chan struct{})
	for range workers {
		go func() {
			<-start
			n, err := s.AddNote(ctx, fullNote("busy"))
			results <- result{n.Number, err}
		}()
	}
	close(start)

	seen := make(map[int]bool, workers)
	for range workers {
		r := <-results
		if r.err != nil {
			t.Errorf("AddNote: %v", r.err)
			continue
		}
		if seen[r.num] {
			t.Errorf("number %d assigned twice", r.num)
		}
		seen[r.num] = true
	}
	for i := 1; i <= workers; i++ {
		if !seen[i] {
			t.Errorf("number %d never assigned", i)
		}
	}
	list, err := s.ListNotes(ctx, "busy")
	must(t, "ListNotes", err)
	if len(list) != workers {
		t.Fatalf("ListNotes len = %d, want %d", len(list), workers)
	}
	for i, x := range list {
		if x.Number != i+1 {
			t.Fatalf("ListNotes[%d].Number = %d", i, x.Number)
		}
	}
}
