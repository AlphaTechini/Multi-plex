package sqlite

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
)

func benchStore(b *testing.B) *Store {
	b.Helper()
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { s.Close() })
	return s
}

func BenchmarkAddNote(b *testing.B) {
	s, ctx := benchStore(b), context.Background()
	n := core.Note{SessionID: "s", Title: "t", Body: "body", Severity: core.SeverityInfo,
		Refs: []string{"a.go:L1-L2"}, At: time.Now()}
	b.ResetTimer()
	for range b.N {
		if _, err := s.AddNote(ctx, n); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPutSession(b *testing.B) {
	s, ctx := benchStore(b), context.Background()
	sess := core.Session{WorkspaceID: "w", Slug: "slug", Task: "task", Agent: "claude-code",
		State: core.StateWorking, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	b.ResetTimer()
	for i := range b.N {
		sess.ID = core.SessionID("s" + strconv.Itoa(i%100)) // mix of inserts and updates
		sess.UpdatedAt = time.Now()
		if err := s.PutSession(ctx, sess); err != nil {
			b.Fatal(err)
		}
	}
}
