package storetest

import (
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

func testSessions(t *testing.T, s store.Store) {
	_, err := s.GetSession(ctx, "missing")
	notFound(t, "GetSession", err)
	must(t, "DeleteSession missing", s.DeleteSession(ctx, "missing"))
	if got, err := s.ListSessions(ctx, ""); err != nil || len(got) != 0 {
		t.Fatalf("ListSessions on empty store = %v, %v", got, err)
	}

	// Insert out of CreatedAt order across two workspaces.
	a3, a1 := fullSession("a3", "wa", 30), fullSession("a1", "wa", 10)
	b2, a2 := fullSession("b2", "wb", 20), fullSession("a2", "wa", 15)
	for _, sess := range []core.Session{a3, b2, a1, a2} {
		must(t, "PutSession", s.PutSession(ctx, sess))
	}

	got, err := s.GetSession(ctx, "a1")
	must(t, "GetSession", err)
	same(t, "GetSession", normSession(got), normSession(a1))
	sameTime(t, "CreatedAt", got.CreatedAt, a1.CreatedAt)
	sameTime(t, "UpdatedAt", got.UpdatedAt, a1.UpdatedAt)

	ids := func(ws core.WorkspaceID) []core.SessionID {
		list, err := s.ListSessions(ctx, ws)
		must(t, "ListSessions", err)
		out := make([]core.SessionID, len(list))
		for i, x := range list {
			out[i] = x.ID
		}
		return out
	}
	same(t, "ListSessions wa", ids("wa"), []core.SessionID{"a1", "a2", "a3"})
	same(t, "ListSessions wb", ids("wb"), []core.SessionID{"b2"})
	same(t, "ListSessions all", ids(""), []core.SessionID{"a1", "a2", "b2", "a3"})
	same(t, "ListSessions unknown", ids("zzz"), []core.SessionID{})

	// Upsert: every field replaced, booleans can go back to false.
	a1.Task, a1.State, a1.StateDetail = "other task", core.StateDone, ""
	a1.Unseen, a1.Follow, a1.AgentPane = false, false, ""
	a1.UpdatedAt = ts(5000)
	must(t, "PutSession upsert", s.PutSession(ctx, a1))
	got, err = s.GetSession(ctx, "a1")
	must(t, "GetSession after upsert", err)
	same(t, "upserted session", normSession(got), normSession(a1))
	same(t, "ListSessions after upsert", ids(""), []core.SessionID{"a1", "a2", "b2", "a3"})

	// Moving a session to another workspace is reflected in the filter.
	a2.WorkspaceID = "wb"
	must(t, "PutSession move", s.PutSession(ctx, a2))
	same(t, "ListSessions wb after move", ids("wb"), []core.SessionID{"a2", "b2"})

	must(t, "DeleteSession", s.DeleteSession(ctx, "a1"))
	_, err = s.GetSession(ctx, "a1")
	notFound(t, "GetSession deleted", err)
	same(t, "ListSessions after delete", ids(""), []core.SessionID{"a2", "b2", "a3"})
}
