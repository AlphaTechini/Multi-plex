package storetest

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

func testWorkspaces(t *testing.T, s store.Store) {
	_, err := s.GetWorkspace(ctx, "missing")
	notFound(t, "GetWorkspace", err)
	_, err = s.WorkspaceByRoot(ctx, "/nowhere")
	notFound(t, "WorkspaceByRoot", err)
	if got, err := s.ListWorkspaces(ctx); err != nil || len(got) != 0 {
		t.Fatalf("ListWorkspaces on empty store = %v, %v", got, err)
	}

	// Insert out of order; List must order by CreatedAt.
	w2, w1, w3 := fullWorkspace("w2", 20), fullWorkspace("w1", 10), fullWorkspace("w3", 30)
	for _, w := range []core.Workspace{w2, w1, w3} {
		must(t, "PutWorkspace", s.PutWorkspace(ctx, w))
	}

	got, err := s.GetWorkspace(ctx, "w1")
	must(t, "GetWorkspace", err)
	same(t, "GetWorkspace", normWorkspace(got), normWorkspace(w1))
	sameTime(t, "CreatedAt", got.CreatedAt, w1.CreatedAt)

	got, err = s.WorkspaceByRoot(ctx, w3.Root)
	must(t, "WorkspaceByRoot", err)
	same(t, "WorkspaceByRoot", normWorkspace(got), normWorkspace(w3))

	list, err := s.ListWorkspaces(ctx)
	must(t, "ListWorkspaces", err)
	if len(list) != 3 {
		t.Fatalf("ListWorkspaces len = %d, want 3", len(list))
	}
	for i, want := range []core.Workspace{w1, w2, w3} {
		same(t, "ListWorkspaces order", normWorkspace(list[i]), normWorkspace(want))
	}

	// Put is an upsert by ID.
	w1.Name, w1.Root = "renamed", "/elsewhere"
	must(t, "PutWorkspace upsert", s.PutWorkspace(ctx, w1))
	got, err = s.GetWorkspace(ctx, "w1")
	must(t, "GetWorkspace after upsert", err)
	same(t, "upserted workspace", normWorkspace(got), normWorkspace(w1))
	if list, _ = s.ListWorkspaces(ctx); len(list) != 3 {
		t.Errorf("upsert changed workspace count to %d", len(list))
	}
	_, err = s.WorkspaceByRoot(ctx, "/repos/w1")
	notFound(t, "WorkspaceByRoot old root", err)
}
