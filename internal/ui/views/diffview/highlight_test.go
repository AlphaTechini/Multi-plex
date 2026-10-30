package diffview

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/highlight/chromahl"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// countingHL counts how many lines are tokenised.
type countingHL struct {
	highlight.Highlighter
	lines int
}

func (c *countingHL) Lines(path, src string) [][]highlight.Token {
	out := c.Highlighter.Lines(path, src)
	c.lines += len(out)
	return out
}

func TestHighlightsOnlyTheWindow(t *testing.T) {
	be := synthBig(4000)
	hl := &countingHL{Highlighter: chromahl.New()}
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	m := newModel(view.Env{Theme: reduced(), Backend: be}, hl, clock.now)
	h := &harness{t: t, m: m, be: be, clock: clock}
	h.send(view.FocusMsg{Focused: true})
	h.send(view.SessionMsg{Session: testSession})
	m.Render(112, 45)
	if hl.lines == 0 {
		t.Fatal("nothing was highlighted")
	}
	// 4,000 diff lines, two sides: a window is a small fraction of that.
	if hl.lines > 1500 {
		t.Fatalf("%d lines tokenised for the first screen of a 4,000-line diff", hl.lines)
	}
	first := hl.lines
	// Scrolling far away highlights there, nothing more.
	for i := 0; i < 30; i++ {
		h.key("ctrl+d")
	}
	m.Render(112, 45)
	if hl.lines == first {
		t.Fatal("the new window was not highlighted")
	}
	if hl.lines > 3000 {
		t.Fatalf("%d lines tokenised after scrolling", hl.lines)
	}
}

func TestSyntaxColoursAppearAfterTokensArrive(t *testing.T) {
	th := theme.DarkPlus()
	be := mockupFake()
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	m := newModel(view.Env{Theme: th, Backend: be}, chromahl.New(), clock.now)
	h := &harness{t: t, m: m, be: be, clock: clock}
	h.send(view.FocusMsg{Focused: true})
	h.send(view.SessionMsg{Session: testSession})
	m.Render(112, 45)
	h.openPath("src/auth/session.go")
	h.clock.advance(time.Second)
	h.send(view.TickMsg{})
	kw := "38;2;86;156;214" // Syntax.Keyword
	if !strings.Contains(m.Render(112, 45), kw) {
		t.Fatalf("keyword colour missing once tokens arrived")
	}
	// Plain until tokens arrive: a fresh model that only gets the diff.
	m2 := newModel(view.Env{Theme: th, Backend: be}, chromahl.New(), clock.now)
	m2.focused = true
	_, cmd := m2.Update(view.SessionMsg{Session: testSession})
	for _, c := range cmd().(tea.BatchMsg) {
		if r := c(); r != nil {
			if _, ok := r.(diffLoadedMsg); ok {
				_, _ = m2.Update(r)
			}
		}
	}
	_, _ = m2.Update(view.SwitchModeMsg{Path: "src/auth/session.go"})
	m2.anim.switching = false
	out := m2.Render(112, 45)
	if strings.Contains(out, kw) {
		t.Fatal("syntax colours drawn before any tokens exist")
	}
	if !strings.Contains(plain(out), "func (s *Session) Refresh") {
		t.Fatal("plain text missing while waiting for tokens")
	}
}

func TestStaleTokensDoNotFlashPlain(t *testing.T) {
	be := mockupFake()
	h := newHarness(t, theme.DarkPlus(), be)
	h.m.Render(112, 45)
	h.openPath("src/auth/session.go")
	h.clock.advance(time.Second)
	h.send(view.TickMsg{})
	kw := "38;2;86;156;214"
	if !strings.Contains(h.render(112, 45), kw) {
		t.Fatal("no highlight before the edit")
	}
	// A line is inserted above: numbers below shift. Rebuild without
	// running commands: the cached tokens are found again by text.
	be.raw = strings.Replace(be.raw, " \tdefer s.mu.Unlock()\n", " \tdefer s.mu.Unlock()\n+\t// note\n", 1)
	be.raw = strings.Replace(be.raw, "@@ -38,13 +38,14 @@", "@@ -38,13 +38,15 @@", 1)
	_, cmd := h.m.Update(view.EventMsg{Topic: "diff.changed"})
	_ = cmd
	_, cmd = h.m.Update(refreshDueMsg{gen: h.m.gen})
	msg := cmd()
	_, _ = h.m.Update(msg)
	if !strings.Contains(h.m.Render(112, 45), kw) {
		t.Fatal("syntax colours vanished while the refresh re-highlights")
	}
}

func TestMultiLineComment(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.selectNew(41, 41)
	h.key("c")
	h.typeText("first line")
	h.key("shift+enter")
	h.typeText("second line")
	out := plain(h.render(112, 45))
	if !strings.Contains(out, "first line") || !strings.Contains(out, "second line") {
		t.Fatalf("multi-line body:\n%s", out)
	}
	h.key("enter")
	if got := h.be.snapshotComments()[0].Body; got != "first line\nsecond line" {
		t.Fatalf("body %q", got)
	}
}

func TestPasteIntoComment(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.selectNew(41, 41)
	h.key("c")
	h.send(tea.PasteMsg{Content: "pasted text"})
	if !strings.Contains(plain(h.render(112, 45)), "pasted text") {
		t.Fatal("paste lost")
	}
}

func TestDragSelects(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.send(click(60, 2+4, 0))
	from := h.m.fv.cursor
	h.send(tea.MouseMotionMsg{X: 60, Y: 2 + 7, Button: tea.MouseLeft})
	if h.m.fv.sel != from || h.m.fv.cursor != from+3 {
		t.Fatalf("drag: sel %d cursor %d (from %d)", h.m.fv.sel, h.m.fv.cursor, from)
	}
	h.send(tea.MouseReleaseMsg{X: 60, Y: 2 + 7, Button: tea.MouseLeft})
	h.send(tea.MouseMotionMsg{X: 60, Y: 2 + 9, Button: tea.MouseLeft})
	if h.m.fv.cursor != from+3 {
		t.Fatal("drag continued after release")
	}
}
