package fake

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

var ctx = context.Background()

func drain(b *Backend, topic string) []json.RawMessage {
	var out []json.RawMessage
	for {
		select {
		case ev := <-b.Events():
			if ev.Topic == topic {
				out = append(out, ev.Data)
			}
		default:
			return out
		}
	}
}

func TestSeed(t *testing.T) {
	b := New()
	defer b.Close()
	ss, err := b.Sessions(ctx, "")
	if err != nil || len(ss) != 3 {
		t.Fatalf("sessions = %v, %v", ss, err)
	}
	want := map[string]core.SessionState{"auth-refactor": core.StateWorking, "api-pagination": core.StateBlocked, "docs-sweep": core.StateDone}
	for _, s := range ss {
		if want[s.Slug] != s.State {
			t.Errorf("%s state = %s", s.Slug, s.State)
		}
	}
	d, err := b.Session(ctx, SessionAuth)
	if err != nil || d.Summary == nil || len(d.Notes) != 3 || len(d.Changes) != 7 || len(d.Conflicts) != 1 {
		t.Fatalf("detail = %+v, %v", d, err)
	}
	pend, _ := b.PendingApprovals(ctx)
	if len(pend) != 1 || pend[0].SessionID != SessionAPI {
		t.Fatalf("pending = %v", pend)
	}
	if specs, _ := b.Specs(ctx, ""); len(specs) != 1 || specs[0].TasksDone != 3 || specs[0].TasksTotal != 7 {
		t.Fatalf("specs = %v", specs)
	}
	if e := Empty; e == nil {
		t.Fatal()
	}
	eb := New(Empty())
	defer eb.Close()
	if ss, _ := eb.Sessions(ctx, ""); len(ss) != 0 {
		t.Fatalf("empty backend has sessions: %v", ss)
	}
}

func TestDiffParses(t *testing.T) {
	b := New()
	defer b.Close()
	for _, id := range []core.SessionID{SessionAuth, SessionAPI, SessionDocs} {
		res, err := b.Diff(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		files, err := diff.Parse(res.Raw)
		if err != nil {
			t.Fatalf("%s: parse: %v", id, err)
		}
		if len(files) != len(res.Changes) || len(files) == 0 {
			t.Fatalf("%s: %d files vs %d changes", id, len(files), len(res.Changes))
		}
		for i, f := range files {
			if f.Added != res.Changes[i].Added || f.Deleted != res.Changes[i].Deleted {
				t.Errorf("%s: counts differ for %s", id, f.Path())
			}
		}
	}
	res, _ := b.Diff(ctx, SessionAuth)
	files, _ := diff.Parse(res.Raw)
	for _, f := range files {
		if f.Path() == "src/auth/session.go" && (len(f.Hunks) != 2 || f.Hunks[0].OldStart != 38 || f.Hunks[1].OldStart != 93) {
			t.Errorf("session.go hunks = %+v", f.Hunks)
		}
	}
}

// The file contents must line up with the diff's line numbers.
func TestFileContentMatchesDiff(t *testing.T) {
	for _, f := range sessionFiles("auth-refactor") {
		newText, ok := fileContent("auth-refactor", f.path, "")
		if !ok {
			t.Fatalf("%s: no content", f.path)
		}
		newLines := strings.Split(strings.TrimSuffix(newText, "\n"), "\n")
		for _, h := range f.hunks {
			for i, l := range h.side(true) {
				at := h.newStart - 1 + i
				if at >= len(newLines) || newLines[at] != l {
					t.Fatalf("%s: new line %d = %q, want %q", f.path, at+1, safeAt(newLines, at), l)
				}
			}
		}
		if f.status == "A" {
			continue
		}
		oldText, _ := fileContent("auth-refactor", f.path, "HEAD")
		oldLines := strings.Split(strings.TrimSuffix(oldText, "\n"), "\n")
		for _, h := range f.hunks {
			for i, l := range h.side(false) {
				at := h.oldStart - 1 + i
				if at >= len(oldLines) || oldLines[at] != l {
					t.Fatalf("%s: old line %d = %q, want %q", f.path, at+1, safeAt(oldLines, at), l)
				}
			}
		}
	}
	// The gap between session.go's hunks is the 42 unchanged lines of the mockup.
	txt, _ := fileContent("auth-refactor", "src/auth/session.go", "")
	if !strings.Contains(strings.Split(txt, "\n")[37], "func (s *Session) Refresh") {
		t.Fatalf("line 38 = %q", strings.Split(txt, "\n")[37])
	}
	old, _ := fileContent("auth-refactor", "src/auth/session.go", "HEAD")
	if got := strings.Split(old, "\n")[91]; !strings.Contains(got, "func (s *Session) Close()") {
		t.Fatalf("old line 92 = %q", got)
	}
}

func safeAt(l []string, i int) string {
	if i < len(l) {
		return l[i]
	}
	return "<eof>"
}

func TestCreateSession(t *testing.T) {
	b := New()
	defer b.Close()
	drain(b, "")
	s, err := b.CreateSession(ctx, protocol.SessionCreateParams{Task: "Make the token cache per-user, please", Role: "planner"})
	if err != nil {
		t.Fatal(err)
	}
	if s.Slug != "make-the-token" || s.Role != "planner" || s.State != core.StateStarting || s.BaseBranch != "main" {
		t.Fatalf("session = %+v", s)
	}
	s2, _ := b.CreateSession(ctx, protocol.SessionCreateParams{Task: "Make the token cache per-user"})
	if s2.Slug == s.Slug {
		t.Fatal("slugs must be unique")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ss, _ := b.Sessions(ctx, "")
		if ss[len(ss)-2].State == core.StateWorking {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("session never started working")
}

func TestApprovalFlow(t *testing.T) {
	b := New()
	defer b.Close()
	drain(b, "")
	d, err := b.RespondApproval(ctx, protocol.ApprovalRespondParams{RequestID: ApprovalID, Outcome: core.Approved})
	if err != nil || d.Outcome != core.Approved {
		t.Fatalf("respond = %+v, %v", d, err)
	}
	if p, _ := b.PendingApprovals(ctx); len(p) != 0 {
		t.Fatalf("still pending: %v", p)
	}
	s, _ := b.Session(ctx, SessionAPI)
	if s.Session.State != core.StateWorking {
		t.Fatalf("state = %s", s.Session.State)
	}
	if got := drain(b, protocol.TopicApprovalDone); len(got) != 1 {
		t.Fatalf("approval.resolved events = %d", len(got))
	}
	if _, err := b.RespondApproval(ctx, protocol.ApprovalRespondParams{RequestID: "nope"}); err == nil {
		t.Fatal("unknown approval must fail")
	}
}

func TestTypedCode(t *testing.T) {
	b := New(WithApprovalMethod("typed-code"))
	defer b.Close()
	if _, err := b.RespondApproval(ctx, protocol.ApprovalRespondParams{RequestID: ApprovalID, Outcome: core.Approved, Code: "bad"}); err == nil {
		t.Fatal("wrong code accepted")
	}
	if _, err := b.RespondApproval(ctx, protocol.ApprovalRespondParams{RequestID: ApprovalID, Outcome: core.Approved, Code: "7K3-Q9"}); err != nil {
		t.Fatalf("right code refused: %v", err)
	}
}

func TestCommentsRoundTrip(t *testing.T) {
	b := New()
	defer b.Close()
	c, err := b.PutComment(ctx, core.Comment{
		SessionID: SessionAuth, Path: "src/auth/session.go", NewStart: 44, NewEnd: 46, Language: "go",
		Snippet: "tok, err := s.cache.Get()", Body: "Wrap with the user ID too.", Status: core.CommentQueued,
	})
	if err != nil || c.ID == "" {
		t.Fatalf("put = %+v, %v", c, err)
	}
	res, err := b.SendComments(ctx, SessionAuth)
	if err != nil || res.Sent != 1 || !strings.Contains(res.Message, "src/auth/session.go:L44-L46") ||
		!strings.Contains(res.Message, "Comment: Wrap with the user ID too.") {
		t.Fatalf("send = %+v, %v", res, err)
	}
	list, _ := b.Comments(ctx, SessionAuth)
	if len(list) != 1 || list[0].Status != core.CommentSent {
		t.Fatalf("comments = %+v", list)
	}
	if err := b.DeleteComment(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ = b.Comments(ctx, SessionAuth); len(list) != 0 {
		t.Fatal("comment not deleted")
	}
}

func TestPaneFramesAndInput(t *testing.T) {
	b := New()
	defer b.Close()
	snap, err := b.PaneSnapshot(ctx, PaneAuth)
	if err != nil || !snap.Full || len(snap.Changed) != snap.Rows {
		t.Fatalf("snapshot = %+v, %v", snap, err)
	}
	var text strings.Builder
	for _, r := range snap.Changed {
		if w := ansi.StringWidth(r.Text); w != snap.Cols {
			t.Fatalf("row %d is %d cells, want %d", r.Y, w, snap.Cols)
		}
		text.WriteString(ansi.Strip(r.Text) + "\n")
	}
	for _, want := range []string{"Read(src/auth/session.go)", "should the cache be per-user or global?", "? for shortcuts"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("transcript lacks %q", want)
		}
	}
	drain(b, "")
	if err := b.PaneInput(ctx, PaneAuth, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	frames := drain(b, protocol.TopicPaneFrame)
	if len(frames) == 0 {
		t.Fatal("typing produced no frame")
	}
	var f protocol.PaneFrame
	_ = json.Unmarshal(frames[len(frames)-1], &f)
	if f.Full || len(f.Changed) >= f.Rows || f.CursorX != 5 {
		t.Fatalf("typing frame = full %v changed %d cursor %d", f.Full, len(f.Changed), f.CursorX)
	}
	// Resize sends a full frame at the new size.
	if err := b.PaneResize(ctx, PaneAuth, 70, 30); err != nil {
		t.Fatal(err)
	}
	frames = drain(b, protocol.TopicPaneFrame)
	_ = json.Unmarshal(frames[len(frames)-1], &f)
	if !f.Full || f.Cols != 70 || f.Rows != 30 {
		t.Fatalf("resize frame = %+v", f)
	}
}

func TestShell(t *testing.T) {
	b := New()
	defer b.Close()
	p, err := b.OpenShell(ctx, SessionAuth, 60, 10)
	if err != nil || p.Kind != core.PaneShell {
		t.Fatalf("shell = %+v, %v", p, err)
	}
	_ = b.PaneInput(ctx, p.ID, []byte("echo hello\r"))
	snap, _ := b.PaneSnapshot(ctx, p.ID)
	var all strings.Builder
	for _, r := range snap.Changed {
		all.WriteString(ansi.Strip(r.Text) + "\n")
	}
	if !strings.Contains(all.String(), "echo hello") || !strings.Contains(all.String(), "hello  ") {
		t.Fatalf("shell screen:\n%s", all.String())
	}
}

func TestStopRestartRemove(t *testing.T) {
	b := New()
	defer b.Close()
	if err := b.StopSession(ctx, SessionAuth); err != nil {
		t.Fatal(err)
	}
	d, _ := b.Session(ctx, SessionAuth)
	if d.Session.State != core.StateIdle {
		t.Fatalf("state = %s", d.Session.State)
	}
	if got := drain(b, protocol.TopicPaneExited); len(got) != 1 {
		t.Fatalf("pane.exited = %d", len(got))
	}
	s, err := b.RestartSession(ctx, SessionAuth)
	if err != nil || s.State != core.StateWorking {
		t.Fatalf("restart = %+v, %v", s, err)
	}
	if err := b.RemoveSession(ctx, SessionDocs); err != nil {
		t.Fatal(err)
	}
	if ss, _ := b.Sessions(ctx, ""); len(ss) != 2 {
		t.Fatalf("sessions = %d", len(ss))
	}
}

func TestTimelineEmits(t *testing.T) {
	b := New(WithTimeline(5 * time.Millisecond))
	defer b.Close()
	seen := map[string]bool{}
	deadline := time.After(3 * time.Second)
	for !(seen[protocol.TopicNote] && seen[protocol.TopicSummary] && seen[protocol.TopicPaneFrame] && seen[protocol.TopicApprovalPending]) {
		select {
		case ev := <-b.Events():
			seen[ev.Topic] = true
		case <-deadline:
			t.Fatalf("timeline events seen: %v", seen)
		}
	}
}

func TestCloseClosesEvents(t *testing.T) {
	b := New(WithTimeline(time.Millisecond))
	_ = b.Close()
	_ = b.Close()
	for range b.Events() {
	}
}
