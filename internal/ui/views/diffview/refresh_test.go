package diffview

import (
	"strings"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

func TestCollapsedRowsExpand(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	out := plain(h.render(112, 45))
	if !strings.Contains(out, "··· 42 unchanged lines ···") || !strings.Contains(out, "··· 37 unchanged lines ···") {
		t.Fatalf("folds missing:\n%s", out)
	}
	fv := h.m.fv
	// Put the cursor on the fold between the hunks and press enter.
	var gap int
	for i, u := range fv.units {
		if u.kind == uFold && fv.rows[u.row].Collapsed == 42 {
			gap = i
		}
	}
	fv.cursor = gap
	if sel := plain(h.render(112, 45)); !strings.Contains(sel, "expand") {
		t.Fatalf("selected fold lacks the expand hint:\n%s", sel)
	}
	h.key("enter")
	out = plain(h.render(112, 45))
	if strings.Contains(out, "42 unchanged") {
		t.Fatalf("fold still collapsed:\n%s", out)
	}
	if !strings.Contains(out, "// filler line 60") {
		t.Fatalf("expanded lines missing:\n%s", out)
	}
	if len(h.m.fv.e.file.Hunks) != 1 {
		t.Fatalf("hunks %d, want the two merged", len(h.m.fv.e.file.Hunks))
	}
	// The cursor lands on the first revealed line.
	_, r := h.m.fv.units[h.m.fv.cursor].lines(&h.m.fv.e.file)
	if r == nil || r.NewNo != 52 {
		t.Fatalf("cursor on %+v, want new line 52", r)
	}
	// Ids shifted: the trailing fold still expands with the new numbering.
	for i, u := range h.m.fv.units {
		if u.kind == uFold {
			h.m.fv.cursor = i
		}
	}
	h.key("enter")
	if strings.Contains(plain(h.render(112, 45)), "unchanged lines") {
		t.Fatal("a fold is left after expanding all of them")
	}
	if got := h.be.reads["src/auth/session.go"]; got != 1 {
		t.Fatalf("file read %d times", got)
	}
}

func TestExpandSurvivesRefresh(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	fv := h.m.fv
	for i, u := range fv.units {
		if u.kind == uFold && fv.rows[u.row].Collapsed == 42 {
			fv.cursor = i
		}
	}
	h.key("enter")
	// Another file changes; session.go is identical: nothing resets.
	h.be.raw += fileDiff("src/extra.go", hunkText(1, 1, "", " a", "-b", "+c"))
	h.refresh()
	if strings.Contains(plain(h.render(112, 45)), "42 unchanged") {
		t.Fatal("expanded gap collapsed again after a refresh")
	}
	// session.go itself changes: the gap is expanded again on the new parse.
	h.be.raw = strings.Replace(h.be.raw, "+\ts.metrics.Refreshes.Inc()", "+\ts.metrics.Refreshes.Add(1)", 1)
	h.be.files["src/auth/session.go"] = strings.Replace(h.be.files["src/auth/session.go"], "Refreshes.Inc()", "Refreshes.Add(1)", 1)
	h.refresh()
	out := plain(h.render(112, 45))
	if strings.Contains(out, "42 unchanged") || !strings.Contains(out, "Refreshes.Add(1)") {
		t.Fatalf("gap not restored on the new parse:\n%s", out)
	}
}

func TestRefreshKeepsFileAndPosition(t *testing.T) {
	be := bigFileFake(60)
	h := newHarness(t, reduced(), be)
	h.render(112, 45)
	h.key("]") // b/next.go
	h.key("[") // a/big.go
	for i := 0; i < 8; i++ {
		h.key("ctrl+d")
	}
	h.render(112, 45)
	top, cursor := h.m.fv.top, h.m.fv.cursor
	cur := h.m.fv.units[cursor].id(&h.m.fv.e.file)
	if top == 0 {
		t.Fatal("test needs a scrolled file")
	}
	// The other file changes; this one is untouched.
	be.raw = strings.Replace(be.raw, "+z", "+zz", 1)
	h.refresh()
	if h.current() != "a/big.go" || h.m.fv.top != top || h.m.fv.cursor != cursor {
		t.Fatalf("refresh moved the view: file %s top %d→%d cursor %d→%d", h.current(), top, h.m.fv.top, cursor, h.m.fv.cursor)
	}
	// This file changes at the bottom: position by line identity stays put.
	be.raw = strings.Replace(be.raw, "+\tnew59 := a + b + c", "+\tnew59 := a + b + c + 1", 1)
	h.refresh()
	if h.current() != "a/big.go" || h.m.fv.top != top {
		t.Fatalf("refresh of this file moved the view: top %d→%d", top, h.m.fv.top)
	}
	if got := h.m.fv.units[h.m.fv.cursor].id(&h.m.fv.e.file); got != cur {
		t.Fatalf("cursor moved from %v to %v", cur, got)
	}
	// A hunk inserted above shifts the line numbers; the cursor follows its line.
	before := h.m.fv.units[h.m.fv.cursor].id(&h.m.fv.e.file)
	be.raw = strings.Replace(be.raw, "@@ -1,8 +1,8 @@", "@@ -1,8 +1,8 @@", 1)
	h.refresh()
	if got := h.m.fv.units[h.m.fv.cursor].id(&h.m.fv.e.file); got != before {
		t.Fatalf("cursor id %v → %v", before, got)
	}
}

func TestRefreshWhenFileDisappearsGoesToNearest(t *testing.T) {
	be := mockupFake()
	h := newHarness(t, reduced(), be)
	h.render(112, 45)
	h.openPath("src/auth/session_test.go")
	be.raw = strings.Replace(be.raw, fileDiff("src/auth/session_test.go",
		hunkText(80, 80, "func TestRefresh(t *testing.T)",
			" \ts := newSession(t)",
			"-\tif err := s.Refresh(ctx); err != nil {",
			"+\tif err := s.Refresh(context.Background()); err != nil {",
			" \t\tt.Fatal(err)",
			" \t}",
		)), "", 1)
	h.refresh()
	if got := h.current(); got != "src/auth/token.go" {
		t.Fatalf("went to %q, want the next file in rail order", got)
	}
	if len(h.m.files) != 6 {
		t.Fatalf("%d files", len(h.m.files))
	}
	// The last file disappearing goes to the previous one.
	h.openPath("src/worker/refresh.go")
	be.raw = be.raw[:strings.Index(be.raw, "diff --git a/src/worker/refresh.go")] + be.raw[strings.Index(be.raw, "diff --git a/docs/auth.md"):]
	h.refresh()
	if h.current() == "src/worker/refresh.go" || h.current() == "" {
		t.Fatalf("went to %q", h.current())
	}
}

func TestDebounceCoalescesEvents(t *testing.T) {
	be := mockupFake()
	h := newHarness(t, reduced(), be)
	base := be.diffs
	var timers int
	for i := 0; i < 20; i++ {
		_, cmd := h.m.Update(view.EventMsg{Topic: protocol.TopicDiffChanged, Data: []byte(`{"session_id":"s1"}`)})
		if cmd != nil {
			timers++
		}
	}
	if timers != 1 {
		t.Fatalf("%d refresh timers for 20 events", timers)
	}
	// Events of another session are ignored.
	h.m.debouncing = false
	if _, cmd := h.m.Update(view.EventMsg{Topic: protocol.TopicDiffChanged, Data: []byte(`{"session_id":"other"}`)}); cmd != nil {
		t.Fatal("event of another session scheduled a refresh")
	}
	if _, cmd := h.m.Update(view.EventMsg{Topic: protocol.TopicSession, Data: []byte(`{}`)}); cmd != nil {
		t.Fatal("unrelated topic scheduled a refresh")
	}
	h.m.debouncing = true
	h.send(refreshDueMsg{gen: h.m.gen})
	if be.diffs != base+1 {
		t.Fatalf("diff loaded %d times, want 1", be.diffs-base)
	}
}

func TestNewHunkHighlightDecays(t *testing.T) {
	be := mockupFake()
	full := be.raw
	be.raw = strings.Replace(full, sessionGoHunks()[1], "", 1)
	th := theme.DarkPlus()
	h := newHarness(t, th, be)
	h.m.Render(112, 45)
	h.openPath("src/auth/session.go")
	be.raw = full
	if h.m.Animating() {
		// The switch animation from openPath may still run; settle it.
		h.clock.advance(time.Second)
		h.send(view.TickMsg{})
	}
	h.refresh()
	if !h.m.Animating() || h.m.anim.fresh != 1 {
		t.Fatalf("fresh highlight not started: animating %v fresh %v", h.m.Animating(), h.m.anim.fresh)
	}
	set := h.m.fresh["src/auth/session.go"]
	if len(set) != 8 {
		t.Fatalf("%d fresh lines, want the 8 lines of the new hunk", len(set))
	}
	first := h.m.anim.fresh
	h.clock.advance(th.Motion.HunkDecay / 2)
	h.send(view.TickMsg{})
	mid := h.m.anim.fresh
	if !(mid < first && mid > 0) {
		t.Fatalf("highlight %v → %v, want a decay", first, mid)
	}
	h.clock.advance(th.Motion.HunkDecay)
	h.send(view.TickMsg{})
	if h.m.Animating() || h.m.anim.fresh != 0 || len(h.m.fresh) != 0 {
		t.Fatalf("highlight did not end: animating %v fresh %v", h.m.Animating(), h.m.anim.fresh)
	}
}

func TestNewHunkReducedMotionIsStaticMark(t *testing.T) {
	be := mockupFake()
	full := be.raw
	be.raw = strings.Replace(full, sessionGoHunks()[1], "", 1)
	h := newHarness(t, reduced(), be)
	h.m.Render(112, 45)
	h.openPath("src/auth/session.go")
	be.raw = full
	h.refresh()
	if h.m.Animating() {
		t.Fatal("reduced motion must not animate")
	}
	if h.m.anim.fresh != 1 {
		t.Fatalf("static mark missing: %v", h.m.anim.fresh)
	}
	if !strings.Contains(plain(h.render(112, 45)), "▎") {
		t.Fatal("no thin accent bar on the new hunk")
	}
	h.key("j")
	if h.m.anim.freshOn {
		t.Fatal("the mark must end when the user moves")
	}
}

func TestFileRemovedStubKeepsStatus(t *testing.T) {
	be := newFake()
	be.changes = []vcs.FileChange{{Path: "img/logo.png", Status: vcs.Modified, Binary: true}}
	be.raw = fileDiff("a.go", hunkText(1, 1, "", " x", "-y", "+z"))
	h := newHarness(t, reduced(), be)
	if len(h.m.files) != 2 {
		t.Fatalf("%d files", len(h.m.files))
	}
}
