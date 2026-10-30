package storetest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

var ctx = context.Background()

// zone is deliberately not UTC so the suite proves times are normalised.
var zone = time.FixedZone("test", 5*3600+1800)

// ts returns a deterministic time n seconds after a fixed origin, in a
// non-UTC zone, with a nanosecond component.
func ts(n int) time.Time {
	return time.Date(2026, 3, 4, 5, 6, 7, 123456789, zone).Add(time.Duration(n) * time.Second)
}

func fullWorkspace(id string, n int) core.Workspace {
	return core.Workspace{ID: core.WorkspaceID(id), Root: "/repos/" + id, Name: "Name " + id, CreatedAt: ts(n)}
}

func fullSession(id, ws string, n int) core.Session {
	return core.Session{
		ID: core.SessionID(id), WorkspaceID: core.WorkspaceID(ws), Slug: "slug-" + id,
		Task: "task \"quoted\" ünïcode", Role: "reviewer", Agent: "claude-code",
		BaseBranch: "main", Branch: "mpx/" + id, WorktreePath: "wt/" + id,
		State: core.StateBlocked, StateDetail: "waiting for approval", AgentPane: core.PaneID("pane-" + id),
		Unseen: true, Follow: true, CreatedAt: ts(n), UpdatedAt: ts(n + 1000),
	}
}

func fullSummary(sid string, n int) core.Summary {
	return core.Summary{
		SessionID: core.SessionID(sid), Done: []string{"a", "b,c", `d"e`},
		FilesChanged: []string{"x/y.go", "z.go"}, Decisions: []string{"use sqlite"},
		OpenQuestions: []string{"why?", "when?"}, Next: "ship it", At: ts(n),
	}
}

func fullNote(sid string) core.Note {
	return core.Note{
		SessionID: core.SessionID(sid), Title: "Heads up", Body: "line1\nline2",
		Severity: core.SeverityDecisionNeeded, Refs: []string{"a.go:L1-L2", "b/c.go:L40-L52"}, At: ts(7),
	}
}

func fullComment(id, sid, path string, newStart, n int) core.Comment {
	return core.Comment{
		ID: id, SessionID: core.SessionID(sid), Path: path, NewStart: newStart, NewEnd: newStart + 3,
		OldStart: newStart - 1, OldEnd: newStart + 2, BaseCommit: "base" + id, HeadCommit: "head" + id,
		Snippet: "snip", Context: "ctx\nctx2", Language: "go", Body: "please fix",
		Status: core.CommentQueued, Changed: true, At: ts(n),
	}
}

func fullCheckpoint(sid string, n int) core.Checkpoint {
	return core.Checkpoint{
		SessionID: core.SessionID(sid), N: n, Ref: "refs/mpx/" + sid + "/1",
		Commit: "0123456789abcdef", Reason: "before edit", At: ts(n),
	}
}

func fullApproval(id, sid string, n int) (core.ApprovalRequest, core.ApprovalDecision) {
	return core.ApprovalRequest{
			ID: id, SessionID: core.SessionID(sid), Kind: core.ApprovalFileWrite, Tool: "Write",
			Command: "rm -rf x", Cwd: "wt/" + sid, Paths: []string{"a.go", "b dir/c.go"},
			Rule: "path-outside", Layer: "path-policy", At: ts(n),
		}, core.ApprovalDecision{
			RequestID: id, Outcome: core.Denied, Message: "nope", Method: "typed-code", At: ts(n + 1),
		}
}

// Normalisers: put every time in UTC so values compare independent of zone.

func normWorkspace(w core.Workspace) core.Workspace {
	w.CreatedAt = w.CreatedAt.UTC()
	return w
}

func normSession(s core.Session) core.Session {
	s.CreatedAt, s.UpdatedAt = s.CreatedAt.UTC(), s.UpdatedAt.UTC()
	return s
}

func normSummary(s core.Summary) core.Summary { s.At = s.At.UTC(); return s }

func normNote(n core.Note) core.Note { n.At = n.At.UTC(); return n }

func normComment(c core.Comment) core.Comment { c.At = c.At.UTC(); return c }

func normCheckpoint(c core.Checkpoint) core.Checkpoint { c.At = c.At.UTC(); return c }

func normAudit(e store.AuditEntry) store.AuditEntry {
	e.Request.At, e.Decision.At = e.Request.At.UTC(), e.Decision.At.UTC()
	return e
}

// same fails the test when got and want differ. Values are compared as JSON,
// which prints readably, distinguishes nil from empty slices, and compares
// times at nanosecond precision (callers normalise both sides to UTC).
func same(t *testing.T, what string, got, want any) {
	t.Helper()
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	if string(g) != string(w) {
		t.Errorf("%s mismatch\n got: %s\nwant: %s", what, g, w)
	}
}

// sameTime asserts two instants are equal after UTC normalisation.
func sameTime(t *testing.T, what string, got, want time.Time) {
	t.Helper()
	if !got.UTC().Equal(want.UTC()) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func notFound(t *testing.T, what string, err error) {
	t.Helper()
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("%s: err = %v, want store.ErrNotFound", what, err)
	}
}

func must(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}
