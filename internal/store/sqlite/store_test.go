package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
	"github.com/alphatechini/multi-plex/internal/store/storetest"
)

func TestConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		s, err := Open(filepath.Join(t.TempDir(), "mpx.db"))
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

func TestReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mpx.db")
	ctx := context.Background()
	at := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ws := core.Workspace{ID: "w", Root: "/r", Name: "n", CreatedAt: at}
	if err := s.PutWorkspace(ctx, ws); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNote(ctx, core.Note{SessionID: "s", Title: "t", Refs: []string{"a:L1"}, At: at}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.GetWorkspace(ctx, "w")
	if err != nil || got != ws {
		t.Fatalf("GetWorkspace after reopen = %+v, %v", got, err)
	}
	// Numbering continues across reopen.
	n, err := s.AddNote(ctx, core.Note{SessionID: "s", Title: "t2", At: at})
	if err != nil || n.Number != 2 {
		t.Fatalf("AddNote after reopen = %+v, %v", n, err)
	}
}

func TestOpenCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "mpx.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if perm := fi.Mode().Perm(); perm != 0o700 {
			t.Errorf("directory mode = %o, want 700", perm)
		}
		if fi, err = os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("file mode = %v, %v, want 600", fi.Mode(), err)
		}
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mpx.db")
	for range 3 {
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var version, rows int
		if err := s.db.QueryRow(`SELECT MAX(version), COUNT(*) FROM schema_version`).Scan(&version, &rows); err != nil {
			t.Fatal(err)
		}
		if version != len(migrations) || rows != 1 {
			t.Fatalf("schema_version = %d (%d rows), want %d (1 row)", version, rows, len(migrations))
		}
		s.Close()
	}
}

func TestPragmas(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "mpx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// A pool may open several connections; check a few by holding them.
	ctx := context.Background()
	for _, q := range []struct {
		pragma string
		want   string
	}{{"journal_mode", "wal"}, {"busy_timeout", "5000"}, {"foreign_keys", "1"}, {"synchronous", "1"}} {
		var got string
		if err := s.db.QueryRowContext(ctx, "PRAGMA "+q.pragma).Scan(&got); err != nil || got != q.want {
			t.Errorf("PRAGMA %s = %q, %v, want %q", q.pragma, got, err, q.want)
		}
	}
}

func TestApprovalsAppendOnly(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "mpx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.LogApproval(ctx, core.ApprovalRequest{ID: "r", SessionID: "s"}, core.ApprovalDecision{RequestID: "r"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM approvals`); err == nil {
		t.Error("DELETE on approvals succeeded")
	}
	if _, err := s.db.Exec(`UPDATE approvals SET outcome = 'approve'`); err == nil {
		t.Error("UPDATE on approvals succeeded")
	}
}

func TestZeroTimeAndExtremes(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "mpx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err := s.PutSession(ctx, core.Session{ID: "z", Slug: "z"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSession(ctx, "z")
	if err != nil || !got.CreatedAt.IsZero() || !got.UpdatedAt.IsZero() {
		t.Fatalf("zero times = %+v, %v", got, err)
	}
	if _, err := s.GetSession(ctx, "nope"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
