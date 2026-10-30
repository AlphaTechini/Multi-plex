package diffview

import (
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/diff"
)

func (h *harness) writeComment(text string) {
	h.t.Helper()
	h.key("c")
	if !h.m.Capturing() {
		h.t.Fatal("c did not open the comment box")
	}
	h.typeText(text)
}

func TestCommentCreateQueues(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.selectNew(44, 46)
	h.writeComment("Wrap with the user ID too")
	box := plain(h.render(112, 45))
	for _, want := range []string{"You", "draft", "L44–L46", "Wrap with the user ID too", "queue", "send now", "cancel"} {
		if !strings.Contains(box, want) {
			t.Fatalf("composer box lacks %q:\n%s", want, box)
		}
	}
	h.key("enter")
	if h.m.Capturing() {
		t.Fatal("enter left the box open")
	}
	saved := h.be.snapshotComments()
	if len(saved) != 1 {
		t.Fatalf("%d comments saved", len(saved))
	}
	c := saved[0]
	if c.Path != "src/auth/session.go" || c.NewStart != 44 || c.NewEnd != 46 || c.OldStart != 44 || c.OldEnd != 46 {
		t.Fatalf("anchor %+v", c)
	}
	if c.Status != core.CommentQueued || c.Body != "Wrap with the user ID too" || c.Language != "go" || c.SessionID != "s1" {
		t.Fatalf("comment %+v", c)
	}
	if c.BaseCommit != "base1" || c.HeadCommit != "head1" {
		t.Fatalf("commits %q %q", c.BaseCommit, c.HeadCommit)
	}
	wantSnip := "\ttok, err := s.cache.Get(ctx, s.userID, s.client.Token)\n\tif err != nil {\n\t\treturn fmt.Errorf(\"refresh token: %w\", err)"
	if c.Snippet != wantSnip {
		t.Fatalf("snippet %q", c.Snippet)
	}
	ctxLines := strings.Split(c.Context, "\n")
	if len(ctxLines) != 9 || ctxLines[0] != "\tif s.tokenValid(time.Now()) {" && !strings.HasSuffix(ctxLines[0], "return nil") {
		t.Fatalf("context %q", c.Context)
	}
	if len(h.toasts) == 0 || !strings.Contains(h.toasts[len(h.toasts)-1].Title, "Comment queued for auth-refactor") {
		t.Fatalf("toasts %+v", h.toasts)
	}
	out := plain(h.render(112, 45))
	if !strings.Contains(out, "queued") || !strings.Contains(out, "Wrap with the user ID too") {
		t.Fatalf("comment not shown under its lines:\n%s", out)
	}
	if strings.Contains(out, "cancel") {
		t.Fatalf("hints still shown after saving")
	}
	// The box hangs under line 46, before line 47.
	i46 := strings.Index(out, "fmt.Errorf")
	iBox := strings.Index(out, "Wrap with")
	i47 := strings.Index(out, "metrics.Refreshes")
	if !(i46 < iBox && iBox < i47) {
		t.Fatalf("box misplaced: %d %d %d", i46, iBox, i47)
	}
}

func TestCommentEscCancelsAndDraft(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.selectNew(44, 44)
	h.writeComment("never mind")
	h.key("esc")
	if h.m.Capturing() || len(h.be.snapshotComments()) != 0 {
		t.Fatal("esc must cancel without saving")
	}
	h.writeComment("keep me private")
	h.key("ctrl+d")
	cs := h.be.snapshotComments()
	if len(cs) != 1 || cs[0].Status != core.CommentDraft {
		t.Fatalf("draft not saved: %+v", cs)
	}
	// Empty text saves nothing.
	h.key("c", "enter")
	if len(h.be.snapshotComments()) != 1 {
		t.Fatal("empty comment saved")
	}
}

func TestCommentSendNow(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.selectNew(41, 41)
	h.writeComment("fix this")
	h.key("ctrl+s")
	if h.be.sent != 1 {
		t.Fatalf("sent %d", h.be.sent)
	}
	if !hasToast(h, "1 comment sent") {
		t.Fatalf("toasts %+v", h.toasts)
	}
}

func hasToast(h *harness, title string) bool {
	for _, t := range h.toasts {
		if strings.Contains(t.Title, title) {
			return true
		}
	}
	return false
}

func TestSendQueuedKey(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.key("S")
	if !hasToast(h, "No queued comments") {
		t.Fatalf("toasts %+v", h.toasts)
	}
	for _, n := range []int{41, 44} {
		h.selectNew(n, n)
		h.writeComment("c" + itoa(n))
		h.key("enter")
	}
	h.toasts = nil
	h.key("S")
	if h.be.sent != 2 || !hasToast(h, "2 comments sent") {
		t.Fatalf("sent %d, toasts %+v", h.be.sent, h.toasts)
	}
	out := plain(h.render(112, 45))
	if strings.Count(out, "sent") < 2 {
		t.Fatalf("comments not shown as sent:\n%s", out)
	}
}

func TestCommentOnDeletedLines(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	fv := h.m.fv
	// Select the deleted line 41 alone.
	var del int = -1
	for i, u := range fv.units {
		if l, _ := u.lines(&fv.e.file); l != nil && l.Kind == diff.Del && l.OldNo == 41 {
			del = i
		}
	}
	fv.cursor, fv.sel = del, -1
	h.writeComment("why remove this?")
	h.key("enter")
	c := h.be.snapshotComments()[0]
	if c.OldStart != 41 || c.OldEnd != 41 || c.NewStart != 41 || c.Snippet != "\tif time.Until(s.expiry) > refreshWindow {" {
		t.Fatalf("deletion comment %+v", c)
	}
	out := plain(h.render(112, 45))
	if !strings.Contains(out, "why remove this?") || !strings.Contains(out, "L41 removed") {
		t.Fatalf("deletion comment not shown:\n%s", out)
	}
	// The box hangs under the deleted line, not the added one below.
	if strings.Index(out, "why remove") > strings.Index(out, "tokenValid(time.Now())") {
		t.Fatal("deletion comment placed under the wrong line")
	}
}

// editSession changes the new content of session.go and the diff, then
// refreshes the view.
func editSession(h *harness, mutate func(lines []string) []string) {
	h.t.Helper()
	be := h.be
	content := strings.Split(strings.TrimSuffix(be.files["src/auth/session.go"], "\n"), "\n")
	content = mutate(content)
	be.files["src/auth/session.go"] = strings.Join(content, "\n") + "\n"
	be.raw = strings.Replace(be.raw, "diff --git a/src/auth/session_test.go", "diff --git a/src/auth/session_test.go", 1)
	h.refresh()
}

func TestCommentRelocatesWhenLinesMove(t *testing.T) {
	be := mockupFake()
	h := newHarness(t, reduced(), be)
	h.m.Render(112, 45)
	h.openPath("src/auth/session.go")
	h.selectNew(44, 46)
	h.writeComment("keep the lookup lock-free")
	h.key("enter")
	// Three lines are added above: the diff changes, the file shifts by 3.
	hs := sessionGoHunks()
	moved := strings.NewReplacer("@@ -38,", "@@ -38,", "+38,", "+38,").Replace(hs[0])
	moved = strings.Replace(moved, " \tdefer s.mu.Unlock()\n", " \tdefer s.mu.Unlock()\n+\t// one\n+\t// two\n+\t// three\n", 1)
	moved = strings.Replace(moved, "@@ -38,13 +38,14 @@", "@@ -38,13 +38,17 @@", 1)
	be.raw = strings.Replace(be.raw, hs[0], moved, 1)
	be.raw = strings.Replace(be.raw, "@@ -93,4 +94,8 @@", "@@ -93,4 +97,8 @@", 1)
	lines := strings.Split(strings.TrimSuffix(be.files["src/auth/session.go"], "\n"), "\n")
	var out []string
	out = append(out, lines[:40]...)
	out = append(out, "\t// one", "\t// two", "\t// three")
	out = append(out, lines[40:]...)
	be.files["src/auth/session.go"] = strings.Join(out, "\n") + "\n"
	h.refresh()
	if h.current() != "src/auth/session.go" {
		t.Fatalf("file changed to %s", h.current())
	}
	var pc *placedComment
	for i := range h.m.fv.placed {
		pc = &h.m.fv.placed[i]
	}
	if pc == nil || pc.outdated || pc.start != 47 || pc.end != 49 {
		t.Fatalf("placed %+v", pc)
	}
	saved := be.snapshotComments()[0]
	if saved.NewStart != 47 || saved.NewEnd != 49 {
		t.Fatalf("relocation not stored: %d-%d", saved.NewStart, saved.NewEnd)
	}
	view := plain(h.render(112, 45))
	if !strings.Contains(view, "L47–L49") || strings.Contains(view, "outdated") {
		t.Fatalf("relocated comment:\n%s", view)
	}
}

func TestCommentOutdatedWhenTextGone(t *testing.T) {
	be := mockupFake()
	h := newHarness(t, reduced(), be)
	h.m.Render(112, 45)
	h.openPath("src/auth/session.go")
	h.selectNew(44, 44)
	h.writeComment("cache this")
	h.key("enter")
	// The agent rewrites line 44 (and its neighbours).
	be.files["src/auth/session.go"] = strings.ReplaceAll(be.files["src/auth/session.go"], "tok, err := s.cache.Get(ctx, s.userID, s.client.Token)", "tok, err := s.fetch(ctx)")
	be.raw = strings.ReplaceAll(be.raw, "+\ttok, err := s.cache.Get(ctx, s.userID, s.client.Token)", "+\ttok, err := s.fetch(ctx)")
	h.refresh()
	var pc *placedComment
	for i := range h.m.fv.placed {
		pc = &h.m.fv.placed[i]
	}
	if pc == nil || !pc.outdated {
		t.Fatalf("comment should be outdated: %+v", pc)
	}
	out := plain(h.render(112, 45))
	if !strings.Contains(out, "outdated") || !strings.Contains(out, "cache this") || !strings.Contains(out, "s.cache.Get(ctx, s.userID, s.client.Token)") {
		t.Fatalf("outdated comment must show the original snippet:\n%s", out)
	}
	// The stored comment stays queued (the status in the daemon is untouched).
	if st := be.snapshotComments()[0].Status; st != core.CommentQueued {
		t.Fatalf("status %s", st)
	}
}

func TestCommentEditAndDelete(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.selectNew(41, 41)
	h.writeComment("first")
	h.key("enter")
	h.key("e")
	if !h.m.Capturing() {
		t.Fatal("e did not edit the comment")
	}
	h.typeText(" second")
	h.key("enter")
	cs := h.be.snapshotComments()
	if len(cs) != 1 || cs[0].Body != "first second" {
		t.Fatalf("edit: %+v", cs)
	}
	h.key("d")
	if len(h.be.snapshotComments()) != 0 {
		t.Fatal("d did not delete the comment")
	}
	if strings.Contains(plain(h.render(112, 45)), "first second") {
		t.Fatal("deleted comment still shown")
	}
}

func TestExistingCommentsLoadAndShow(t *testing.T) {
	be := mockupFake()
	be.comments = []core.Comment{{
		ID: "x1", SessionID: "s1", Path: "src/auth/session.go", NewStart: 48, NewEnd: 48,
		Snippet: "\ts.metrics.Refreshes.Inc()", Context: "\t}\n\ts.metrics.Refreshes.Inc()\n\ts.token, s.expiry = tok.Value, tok.Expiry",
		Body: "Count failures too.", Status: core.CommentSent,
	}}
	h := newHarness(t, reduced(), be)
	h.m.Render(112, 45)
	h.openPath("src/auth/session.go")
	out := plain(h.render(112, 45))
	if !strings.Contains(out, "Count failures too.") || !strings.Contains(out, "sent") {
		t.Fatalf("existing comment missing:\n%s", out)
	}
}
