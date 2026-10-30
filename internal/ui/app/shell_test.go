package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend/fake"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// assertRect fails unless s is exactly w columns by h rows.
func assertRect(t *testing.T, s string, w, h int) {
	t.Helper()
	lines := strings.Split(s, "\n")
	if len(lines) != h {
		t.Fatalf("%d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != w {
			t.Fatalf("line %d is %d cells wide, want %d: %q", i, got, w, ansi.Strip(l))
		}
	}
}

func TestFirstScreen(t *testing.T) {
	h := newHarness(t, 180, 48)
	out := h.plain()
	assertRect(t, h.screen(), 180, 48)
	for _, want := range []string{
		"SESSIONS", "auth-refactor", "needs approval", "done · 6 files", "token-refresh", "AGENTS.md",
		"Overview", "Term", "Read(src/auth/session.go)", "1 approval · api-pagination", "palette",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q", want)
		}
	}
}

// Layout widths at the three reference sizes (and the edges between the
// responsive rules) add up to the terminal width, and every line of every
// overlay state is exactly that wide.
func TestLayoutWidths(t *testing.T) {
	sizes := [][2]int{{80, 24}, {110, 32}, {138, 40}, {175, 40}, {176, 44}, {180, 48}, {240, 60}}
	for _, sz := range sizes {
		w, hgt := sz[0], sz[1]
		for _, mode := range modeOrder {
			l := computeLayout(layoutInput{w: w, h: hgt, mode: mode})
			if l.tooSmall {
				t.Fatalf("%dx%d reported too small", w, hgt)
			}
			if l.sidebarW+l.mainW+l.agentW != w {
				t.Errorf("%dx%d mode %d: %d+%d+%d != %d", w, hgt, mode, l.sidebarW, l.mainW, l.agentW, w)
			}
			if l.mainW < mainMinDocked && !l.agentRail {
				t.Errorf("%dx%d: main %d too narrow", w, hgt, l.mainW)
			}
		}
		h := newHarness(t, w, hgt)
		assertRect(t, h.screen(), w, hgt)
		h.keys("ctrl+\\", "?")
		assertRect(t, h.screen(), w, hgt)
		h.key("esc")
		h.keys("ctrl+\\", "n")
		assertRect(t, h.screen(), w, hgt)
		h.key("esc")
		h.keys("ctrl+\\", "p")
		assertRect(t, h.screen(), w, hgt)
		h.key("esc")
		h.keys("ctrl+\\", "!")
		assertRect(t, h.screen(), w, hgt)
		h.key("m")
		assertRect(t, h.screen(), w, hgt)
		h.key("esc")
		h.keys("ctrl+\\", "d")
		assertRect(t, h.screen(), w, hgt)
		h.keys("ctrl+\\", "a")
		assertRect(t, h.screen(), w, hgt)
		h.keys("ctrl+\\", "z")
		assertRect(t, h.screen(), w, hgt)
	}
	// The mockups' geometry.
	l := computeLayout(layoutInput{w: 180, h: 48, mode: view.ModeOverview})
	if l.sidebarW != 28 || l.agentW != 60 || l.mainW != 92 {
		t.Errorf("180: %d/%d/%d", l.sidebarW, l.mainW, l.agentW)
	}
	l = computeLayout(layoutInput{w: 110, h: 32, mode: view.ModeOverview})
	if l.sidebarW != 6 || l.agentW != 3 || !l.sidebarRail || !l.agentRail {
		t.Errorf("110: %d/%d/%d", l.sidebarW, l.mainW, l.agentW)
	}
	l = computeLayout(layoutInput{w: 180, h: 48, mode: view.ModeDiff})
	if !l.sidebarRail || l.sidebarW != 6 {
		t.Errorf("diff mode must collapse the sidebar: %+v", l)
	}
}

func TestTooSmall(t *testing.T) {
	h := newHarness(t, 79, 30)
	out := h.plain()
	if !strings.Contains(out, "Terminal too small") || !strings.Contains(out, "80×24") || !strings.Contains(out, "79×30") {
		t.Fatalf("too small screen:\n%s", out)
	}
	assertRect(t, h.screen(), 79, 30)
	h.send(tea.WindowSizeMsg{Width: 120, Height: 23})
	if !strings.Contains(h.plain(), "Terminal too small") {
		t.Fatal("23 rows must be too small")
	}
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	if strings.Contains(h.plain(), "Terminal too small") {
		t.Fatal("80x24 is supported")
	}
	assertRect(t, h.screen(), 80, 24)
}

func TestWindowTitle(t *testing.T) {
	h := newHarness(t, 120, 40)
	if got := h.m.View().WindowTitle; got != "multiplex · multi-plex" {
		t.Fatalf("title = %q", got)
	}
	v := h.m.View()
	if !v.AltScreen || v.MouseMode != tea.MouseModeCellMotion || !v.ReportFocus {
		t.Fatalf("view flags: alt=%v mouse=%v focus=%v", v.AltScreen, v.MouseMode, v.ReportFocus)
	}
}

// With the agent panel focused every key goes to the agent's terminal,
// except the prefix; the prefix then takes one command key.
func TestPrefixRouting(t *testing.T) {
	h := newHarness(t, 180, 48)
	if h.m.focus != regionMain {
		t.Fatalf("initial focus = %v", h.m.focus)
	}
	h.keys("ctrl+\\", "l") // focus right: the agent panel
	if h.m.focus != regionAgent {
		t.Fatalf("focus after prefix l = %v", h.m.focus)
	}
	before := h.inputCalls()
	h.typeText("hi")
	h.key("enter")
	h.key("p") // a plain p, not the palette
	h.settle()
	if len(h.m.overlays) != 0 {
		t.Fatal("a plain key must not open the palette while the agent is focused")
	}
	if n := h.inputCalls() - before; n < 1 {
		t.Fatalf("keys did not reach the agent pane (%d PaneInput calls)", n)
	}
	// Prefix + p opens the palette; the key is not sent to the agent.
	before = h.inputCalls()
	h.keys("ctrl+\\", "p")
	if len(h.m.overlays) != 1 {
		t.Fatal("prefix p must open the palette")
	}
	h.key("esc")
	h.settle()
	if h.inputCalls() != before {
		t.Fatal("prefix keys leaked into the agent")
	}
	// Prefix twice sends the key itself.
	h.keys("ctrl+\\", "ctrl+\\")
	h.settle()
	if h.inputCalls() == before {
		t.Fatal("prefix prefix must forward the key to the terminal")
	}
	// Esc cancels a pending prefix and is not sent anywhere.
	before = h.inputCalls()
	h.keys("ctrl+\\", "esc")
	h.settle()
	if h.m.prefixOn || h.inputCalls() != before {
		t.Fatal("esc must cancel the prefix silently")
	}
	// The status bar shows the cheat sheet while the prefix is pending.
	h.key("ctrl+\\")
	if out := h.plain(); !strings.Contains(out, "PREFIX") || !strings.Contains(out, "palette") {
		t.Fatalf("no prefix hints:\n%s", out)
	}
	h.key("esc")
}

func (h *harness) inputCalls() int {
	n := 0
	for _, c := range h.be.Calls() {
		if c == "PaneInput" {
			n++
		}
	}
	return n
}

func TestFocusMovesAndAccentFrame(t *testing.T) {
	h := newHarness(t, 180, 48)
	frame := func() string { return h.screen() }
	focusBlue := kit.ToneColor(h.m.th, kit.ToneAccent)
	_ = focusBlue
	for _, step := range []struct {
		keys []string
		want region
	}{
		{[]string{"ctrl+\\", "h"}, regionSidebar},
		{[]string{"ctrl+\\", "l"}, regionMain},
		{[]string{"ctrl+\\", "l"}, regionAgent},
		{[]string{"ctrl+\\", "l"}, regionAgent},
		{[]string{"ctrl+\\", "left"}, regionMain},
		{[]string{"ctrl+\\", "right"}, regionAgent},
	} {
		h.keys(step.keys...)
		if h.m.focus != step.want {
			t.Fatalf("after %v focus = %v, want %v", step.keys, h.m.focus, step.want)
		}
		// Exactly one region carries the focus colour on its top-left corner.
		count := strings.Count(frame(), "\x1b[38;2;55;148;255m╭")
		if count != 1 {
			t.Fatalf("after %v: %d focused frames", step.keys, count)
		}
	}
}

func TestSidebarNavigationSelectsSessions(t *testing.T) {
	spy := installSpy(t, view.ModeOverview)
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "h")
	if h.m.focus != regionSidebar {
		t.Fatal("focus not on sidebar")
	}
	h.key("j")
	if h.m.cur != fake.SessionAPI {
		t.Fatalf("j selected %s", h.m.cur)
	}
	h.key("j")
	if h.m.cur != fake.SessionDocs {
		t.Fatalf("j selected %s", h.m.cur)
	}
	h.settle()
	// The unseen docs-sweep was marked seen on the daemon.
	found := false
	for _, c := range h.be.Calls() {
		if c == "MarkSeen" {
			found = true
		}
	}
	if !found {
		t.Fatal("selecting an unseen session must mark it seen")
	}
	h.key("k")
	h.key("enter") // opens the session and focuses the main view
	if h.m.focus != regionMain || h.m.cur != fake.SessionAPI {
		t.Fatalf("enter: focus %v cur %s", h.m.focus, h.m.cur)
	}
	var told []core.SessionID
	for _, m := range spy.log {
		if sm, ok := m.(view.SessionMsg); ok && sm.Session != nil {
			told = append(told, sm.Session.ID)
		}
	}
	want := []core.SessionID{fake.SessionAuth, fake.SessionAPI, fake.SessionDocs, fake.SessionAPI}
	if len(told) < len(want) {
		t.Fatalf("view was told %v", told)
	}
	got := told[len(told)-len(want):]
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("view was told %v, want ...%v", told, want)
		}
	}
}

func TestSidebarClickAndTabClick(t *testing.T) {
	h := newHarness(t, 180, 48)
	// Sidebar: session items start at screen row 3 (frame, blank, header).
	click := func(x, y int) {
		h.send(tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}))
		h.settle()
	}
	click(8, 5) // api-pagination (rows 5 and 6)
	if h.m.cur != fake.SessionAPI || h.m.focus != regionMain {
		t.Fatalf("click: cur %s focus %v", h.m.cur, h.m.focus)
	}
	click(8, 8) // docs-sweep
	if h.m.cur != fake.SessionDocs {
		t.Fatalf("click: cur %s", h.m.cur)
	}
	// Tabs: " Overview " starts at column 28+3.
	click(28+3+len(" Overview ")+1+2, 0)
	if h.m.mode != view.ModeDiff {
		t.Fatalf("tab click: mode %v", h.m.mode)
	}
	x0 := h.m.lay.x(regionMain) // the sidebar became a rail in Diff mode
	click(x0+3+2, 0)
	if h.m.mode != view.ModeOverview {
		t.Fatalf("tab click: mode %v", h.m.mode)
	}
	// Clicking the agent panel focuses it.
	click(150, 10)
	if h.m.focus != regionAgent {
		t.Fatalf("agent click: focus %v", h.m.focus)
	}
	// Clicking a steering entry opens it in the Code view.
	spy := installSpy(t, view.ModeCode)
	h.m.views = map[view.Mode]view.View{}
	h.m.told = map[view.Mode]core.SessionID{}
	idx := h.m.sbIndexOfSteer("CLAUDE.md")
	lines := h.m.sbLines(26)
	row := -1
	for i, l := range lines {
		if l.item == idx {
			row = i + 1
		}
	}
	click(8, row)
	if h.m.mode != view.ModeCode {
		t.Fatalf("steering click: mode %v", h.m.mode)
	}
	var sw *view.SwitchModeMsg
	for _, m := range spy.log {
		if s, ok := m.(view.SwitchModeMsg); ok {
			sw = &s
		}
	}
	if sw == nil || sw.Path != "CLAUDE.md" {
		t.Fatalf("code view was not asked to open CLAUDE.md: %+v", spy.log)
	}
}

func TestModesViaPrefixAndTab(t *testing.T) {
	h := newHarness(t, 180, 48)
	for i, mode := range modeOrder {
		h.keys("ctrl+\\", itoa(i+1))
		if h.m.mode != mode {
			t.Fatalf("prefix %d: mode %v", i+1, h.m.mode)
		}
	}
	for _, c := range []struct {
		key  string
		mode view.Mode
	}{{"o", view.ModeOverview}, {"d", view.ModeDiff}, {"c", view.ModeCode}, {"s", view.ModeSpecs}, {"t", view.ModeTerm}} {
		h.keys("ctrl+\\", c.key)
		if h.m.mode != c.mode {
			t.Fatalf("prefix %s: mode %v", c.key, h.m.mode)
		}
	}
	h.keys("ctrl+\\", "o")
	// tab cycles when the view is not capturing (main focused, placeholder view).
	h.key("tab")
	if h.m.mode != view.ModeDiff {
		t.Fatalf("tab: mode %v", h.m.mode)
	}
	h.key("shift+tab")
	if h.m.mode != view.ModeOverview {
		t.Fatalf("shift+tab: mode %v", h.m.mode)
	}
	// A capturing view gets tab itself.
	spy := installSpy(t, view.ModeOverview)
	spy.capturing = true
	h.m.views = map[view.Mode]view.View{}
	h.m.told = map[view.Mode]core.SessionID{}
	h.keys("ctrl+\\", "d")
	h.keys("ctrl+\\", "o")
	h.key("tab")
	if h.m.mode != view.ModeOverview {
		t.Fatalf("tab in a capturing view changed the mode to %v", h.m.mode)
	}
	if !spy.saw("tab") {
		t.Fatal("capturing view did not get tab")
	}
}

func TestEventsRouting(t *testing.T) {
	spy := installSpy(t, view.ModeOverview)
	h := newHarness(t, 180, 48)

	// pane.frame updates the agent terminal.
	h.be.Emit(protocol.TopicPaneFrame, protocol.PaneFrame{
		PaneID: fake.PaneAuth, Cols: 58, Rows: 46,
		Changed: []protocol.Row{{Y: 0, Text: kit.Fit("  HELLO FROM THE AGENT", 58)}},
	})
	// note.added bumps the sidebar badge.
	h.be.Emit(protocol.TopicNote, core.Note{ID: "n9", SessionID: fake.SessionAuth, Title: "x"})
	// session.updated changes the sidebar entry.
	s := *h.m.sessionByID(fake.SessionDocs)
	s.State, s.StateDetail = core.StateError, "agent crashed"
	h.be.Emit(protocol.TopicSession, s)
	h.pump()
	out := h.plain()
	if !strings.Contains(out, "HELLO FROM THE AGENT") {
		t.Error("pane frame not rendered")
	}
	if h.m.notes[fake.SessionAuth] != 4 {
		t.Errorf("note count = %d", h.m.notes[fake.SessionAuth])
	}
	if !strings.Contains(out, "agent crashed") || !strings.Contains(out, "docs-sweep failed") {
		t.Errorf("session error not shown:\n%s", out)
	}
	// Everything is forwarded to the views as view.EventMsg.
	topics := map[string]bool{}
	for _, m := range spy.log {
		if e, ok := m.(view.EventMsg); ok {
			topics[e.Topic] = true
		}
	}
	for _, want := range []string{protocol.TopicNote, protocol.TopicSession} {
		if !topics[want] {
			t.Errorf("view did not get %s", want)
		}
	}
	// Pane frames go only to views that draw a pane (Term).
	if topics[protocol.TopicPaneFrame] {
		t.Error("a view that draws no pane was sent pane.frame")
	}
	// session.removed drops the entry and moves the selection.
	h.be.Emit(protocol.TopicSessionRemoved, protocol.SessionRef{SessionID: fake.SessionAuth})
	h.pump()
	if h.m.sessionByID(fake.SessionAuth) != nil || h.m.cur == fake.SessionAuth || h.m.cur == "" {
		t.Fatalf("removed session still selected: %q", h.m.cur)
	}
}

func TestToasts(t *testing.T) {
	h := newHarness(t, 180, 48)
	for _, s := range []string{"toast-one", "toast-two", "toast-three", "toast-four"} {
		h.be.Emit(protocol.TopicToast, protocol.Toast{Level: "info", Text: s})
	}
	h.pump()
	if len(h.m.toasts) != 3 {
		t.Fatalf("%d toasts, want 3 (oldest pushed out)", len(h.m.toasts))
	}
	out := h.plain()
	if strings.Contains(out, "toast-one") {
		t.Error("oldest toast still shown")
	}
	for _, want := range []string{"toast-two", "toast-three", "toast-four"} {
		if !strings.Contains(out, want) {
			t.Errorf("toast %q missing", want)
		}
	}
	assertRect(t, h.screen(), 180, 48)
	// Info toasts expire; warnings stay.
	h.be.Emit(protocol.TopicToast, protocol.Toast{Level: "warning", Text: "careful"})
	h.pump()
	for _, ts := range h.m.toasts {
		if ts.title == "careful" && !ts.sticky {
			t.Error("warning toast must be sticky")
		}
		if ts.title == "toast-four" && ts.sticky {
			t.Error("info toast must expire")
		}
	}
	h.m.Update(toastExpireMsg{id: h.m.toasts[0].id})
	if len(h.m.toasts) != 2 {
		t.Fatalf("expire left %d toasts", len(h.m.toasts))
	}
	// Clicking dismisses.
	blocks, xs, ys := h.m.toastRects()
	if len(blocks) == 0 {
		t.Fatal("no toast rects")
	}
	h.send(tea.MouseClickMsg(tea.Mouse{X: xs[0] + 3, Y: ys[0] + 1, Button: tea.MouseLeft}))
	if len(h.m.toasts) != 1 {
		t.Fatalf("click left %d toasts", len(h.m.toasts))
	}
	h.keys("ctrl+\\", ".")
	if len(h.m.toasts) != 0 {
		t.Fatal("prefix . must dismiss toasts")
	}
}

func TestAgentDrawerAndZoom(t *testing.T) {
	h := newHarness(t, 110, 32)
	if !h.m.lay.agentRail || h.m.lay.drawer {
		t.Fatal("110 wide must start with the agent rail")
	}
	h.keys("ctrl+\\", "a")
	if !h.m.lay.drawer || h.m.focus != regionAgent || h.m.lay.drawerW != 60 {
		t.Fatalf("drawer: %+v focus %v", h.m.lay, h.m.focus)
	}
	out := h.plain()
	if !strings.Contains(out, "Read(src/auth/session.go)") {
		t.Fatalf("drawer does not show the agent:\n%s", out)
	}
	assertRect(t, h.screen(), 110, 32)
	h.settle()
	if cols, _ := h.m.agentPaneOf(h.m.session()).Size(); cols != 58 {
		t.Fatalf("agent pane size = %d cols, want 58 (drawer inner)", cols)
	}
	h.keys("ctrl+\\", "h") // focus leaves: the drawer closes
	if h.m.lay.drawer {
		t.Fatal("drawer must close when focus leaves it")
	}
	h.keys("ctrl+\\", "l") // moving right onto the rail opens it again
	if !h.m.lay.drawer {
		t.Fatal("prefix l onto the rail must open the drawer")
	}
	h.keys("ctrl+\\", "a")
	if h.m.lay.drawer || h.m.focus == regionAgent {
		t.Fatal("prefix a must close the drawer")
	}

	// Zoom: the focused region fills the screen.
	h.keys("ctrl+\\", "z")
	if h.m.lay.mainW != 110 || h.m.lay.sidebarW != 0 {
		t.Fatalf("zoom: %+v", h.m.lay)
	}
	assertRect(t, h.screen(), 110, 32)
	h.keys("ctrl+\\", "z")
	if h.m.lay.mainW == 110 {
		t.Fatal("second zoom must restore")
	}

	// Wide terminal: a docks, a again collapses to the rail.
	h = newHarness(t, 180, 48)
	h.keys("ctrl+\\", "a")
	if !h.m.lay.agentRail || h.m.focus == regionAgent {
		t.Fatalf("a must collapse the docked panel: %+v", h.m.lay)
	}
	assertRect(t, h.screen(), 180, 48)
	h.keys("ctrl+\\", "a")
	if h.m.lay.agentRail || h.m.focus != regionAgent {
		t.Fatalf("a must dock it again and focus it: %+v focus %v", h.m.lay, h.m.focus)
	}
	h.keys("ctrl+\\", "w")
	if got := h.m.lay.order; got[1] != regionAgent {
		t.Fatalf("swap: order %v", got)
	}
	assertRect(t, h.screen(), 180, 48)
}

func TestSessionActions(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "]")
	if h.m.cur != fake.SessionAPI {
		t.Fatalf("] selected %s", h.m.cur)
	}
	h.keys("ctrl+\\", "[")
	h.keys("ctrl+\\", "[")
	if h.m.cur != fake.SessionDocs {
		t.Fatalf("[ must wrap to the last session, got %s", h.m.cur)
	}
	h.keys("ctrl+\\", "[")
	h.keys("ctrl+\\", "[")
	if h.m.cur != fake.SessionAuth {
		t.Fatalf("cur = %s", h.m.cur)
	}
	h.keys("ctrl+\\", "f")
	h.keys("ctrl+\\", "x")
	h.settle()
	calls := strings.Join(h.be.Calls(), ",")
	for _, want := range []string{"SetFollow", "StopSession"} {
		if !strings.Contains(calls, want) {
			t.Errorf("%s not called (%s)", want, calls)
		}
	}
	h.pump()
	if s := h.m.session(); s.State != core.StateIdle {
		t.Fatalf("state after stop = %s", s.State)
	}
	// The stopped agent shows the exit notice; r on the agent restarts.
	h.keys("ctrl+\\", "l", "l")
	if h.m.focus != regionAgent {
		t.Fatalf("focus %v", h.m.focus)
	}
	if out := h.plain(); !strings.Contains(out, "agent exited") || !strings.Contains(out, "r restart") {
		t.Fatalf("no exit notice:\n%s", out)
	}
	h.key("r")
	h.settle()
	h.pump()
	if s := h.m.session(); s.State != core.StateWorking {
		t.Fatalf("state after r = %s", s.State)
	}
}

func TestEmptyState(t *testing.T) {
	h := newHarness(t, 180, 48, fake.Empty())
	out := h.plain()
	for _, want := range []string{"Welcome", "No sessions yet", "none yet", "no sessions", "new session", "command palette", "all keys", "Agent"} {
		if !strings.Contains(out, want) {
			t.Errorf("empty screen lacks %q", want)
		}
	}
	assertRect(t, h.screen(), 180, 48)
	h.key("n") // main view focused on the welcome screen
	if len(h.m.overlays) != 1 {
		t.Fatal("n must open the new-session dialog")
	}
	if _, ok := h.m.overlays[0].(*newSessionOverlay); !ok {
		t.Fatalf("overlay = %T", h.m.overlays[0])
	}
}

func TestIdleDoesNotTick(t *testing.T) {
	installSpy(t, view.ModeOverview) // a view that never animates
	h := newHarness(t, 180, 48)
	if !h.m.needsTick() {
		t.Fatal("a working session animates")
	}
	h.m.blurred = true
	if h.m.needsTick() {
		t.Fatal("a blurred window must not tick")
	}
	h.m.blurred = false
	h.key("ctrl+\\")
	h.key("p")
	if h.m.needsTick() {
		t.Fatal("an open overlay freezes the screen behind it")
	}
	h.key("esc")
	h.m.th = h.m.th.WithReducedMotion(true)
	if h.m.needsTick() {
		t.Fatal("reduced motion without animating views must not tick")
	}
	// A quiet workspace: nothing working, no approvals.
	q := newHarness(t, 180, 48, fake.Empty())
	if q.m.needsTick() || q.m.tickCmd() != nil {
		t.Fatal("an idle app must not tick")
	}
	// The tick advances the spinner frame and re-arms only while needed.
	h.m.th = h.m.th.WithReducedMotion(false)
	h.m.tickArmed = false
	_, cmd := h.m.Update(tickMsg{})
	if h.m.frame != 1 || cmd == nil || !h.m.tickArmed {
		t.Fatalf("tick: frame %d armed %v", h.m.frame, h.m.tickArmed)
	}
}

func TestSpinnerAdvances(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.m.frame = 0
	a := h.plain()
	h.m.frame = 3
	b := h.plain()
	if a == b {
		t.Fatal("spinner frame must change the screen")
	}
	h.m.th = h.m.th.WithReducedMotion(true)
	if !strings.Contains(h.plain(), "◐") {
		t.Fatal("reduced motion must show the static glyph")
	}
}

func TestBenchFirstFrame(t *testing.T) {
	be := fake.New()
	defer be.Close()
	m := newModel(Options{Backend: be, BenchFirstFrame: true}, themeForTest(), demoProbe)
	m.noPump = true
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if cmd == nil {
		t.Fatal("no bench command")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case <-done:
		t.Fatal("bench finished before the first frame was rendered")
	case <-time.After(50 * time.Millisecond):
	}
	_ = m.View()
	select {
	case msg := <-done:
		if _, ok := msg.(benchDoneMsg); !ok {
			t.Fatalf("msg = %T", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("bench never finished")
	}
}

func TestEventPumpBatches(t *testing.T) {
	be := fake.New()
	defer be.Close()
	m := newModel(Options{Backend: be}, themeForTest(), demoProbe)
	for range 5 {
		be.Emit(protocol.TopicToast, protocol.Toast{Level: "info", Text: "x"})
	}
	msg := m.waitEvents()()
	em, ok := msg.(eventsMsg)
	if !ok || len(em.events) != 5 {
		t.Fatalf("pump = %#v", msg)
	}
	be.Close()
	if msg := m.waitEvents()(); !msg.(eventsMsg).closed {
		t.Fatal("closed channel not reported")
	}
}

func TestStatusBarApprovalPillClick(t *testing.T) {
	h := newHarness(t, 180, 48)
	plain := h.plain()
	lines := strings.Split(plain, "\n")
	status := lines[len(lines)-1]
	x := ansi.StringWidth(status[:strings.Index(status, "1 approval")]) + 3
	h.send(tea.MouseClickMsg(tea.Mouse{X: x, Y: 47, Button: tea.MouseLeft}))
	if len(h.m.overlays) != 1 {
		t.Fatalf("pill click opened %d overlays", len(h.m.overlays))
	}
	if _, ok := h.m.overlays[0].(*approvalOverlay); !ok {
		t.Fatalf("overlay = %T", h.m.overlays[0])
	}
}

func TestPasteGoesToFocusedPane(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.keys("ctrl+\\", "l")
	before := h.inputCalls()
	h.send(tea.PasteMsg{Content: "pasted text"})
	h.settle()
	if h.inputCalls() == before {
		t.Fatal("paste did not reach the agent")
	}
}

func TestJSONRoundTripOfEvents(t *testing.T) {
	// handleEvent tolerates garbage payloads.
	h := newHarness(t, 180, 48)
	for _, topic := range []string{protocol.TopicSession, protocol.TopicPaneFrame, protocol.TopicNote, protocol.TopicApprovalPending, protocol.TopicToast} {
		h.send(eventsMsg{events: []backendEvent{{Topic: topic, Data: json.RawMessage(`{bad`)}}})
	}
	assertRect(t, h.screen(), 180, 48)
}

// The region caches must never show stale content: after every step of a
// scenario the cached render equals a render with caching off.
func TestCacheMatchesUncached(t *testing.T) {
	h := newHarness(t, 180, 48)
	check := func(step string) {
		t.Helper()
		cached := h.m.render()
		h.m.noCache = true
		fresh := h.m.render()
		h.m.noCache = false
		if cached != fresh {
			t.Fatalf("%s: cached render differs from a fresh one", step)
		}
	}
	check("start")
	steps := []func(){
		func() { h.keys("ctrl+\\", "h") },
		func() { h.key("j") },
		func() { h.key("j") },
		func() { h.keys("ctrl+\\", "d") },
		func() { h.keys("ctrl+\\", "t") },
		func() { h.settle() },
		func() { h.keys("ctrl+\\", "l") },
		func() {
			h.be.Emit(protocol.TopicPaneFrame, protocol.PaneFrame{PaneID: fake.PaneDocs, Cols: 58, Rows: 46,
				Changed: []protocol.Row{{Y: 3, Text: kit.Fit("  STREAMED ROW", 58)}}})
			h.pump()
		},
		func() { h.m.frame++; h.m.Update(tickMsg{}) },
		func() { h.be.Emit(protocol.TopicNote, core.Note{ID: "nn", SessionID: fake.SessionDocs}); h.pump() },
		func() { h.keys("ctrl+\\", "z") },
		func() { h.send(tea.WindowSizeMsg{Width: 120, Height: 36}) },
		func() { h.keys("ctrl+\\", "a") },
		func() { h.keys("ctrl+\\", "o") },
	}
	for i, step := range steps {
		step()
		check("step " + itoa(i))
	}
}

func TestStreamingOnlyRedrawsAgent(t *testing.T) {
	h := newHarness(t, 180, 48)
	h.render()
	sb, mainOut := h.m.sbCache.out, h.m.mainCache.out
	h.be.Emit(protocol.TopicPaneFrame, protocol.PaneFrame{PaneID: fake.PaneAuth, Cols: 58, Rows: 46,
		Changed: []protocol.Row{{Y: 5, Text: kit.Fit("  streaming…", 58)}}})
	h.pump()
	h.render()
	if h.m.sbCache.out != sb || h.m.mainCache.out != mainOut {
		t.Fatal("a pane frame must not invalidate the sidebar or the main panel")
	}
	if !strings.Contains(h.plain(), "streaming…") {
		t.Fatal("frame not drawn")
	}
}

func (h *harness) render() { _ = h.m.render() }

func TestSessionsGroupedByWorkspace(t *testing.T) {
	h := newHarness(t, 180, 48)
	if strings.Contains(h.plain(), "MULTI-PLEX") {
		t.Fatal("a single workspace needs no group header")
	}
	other := core.Session{
		ID: "ses_other", WorkspaceID: "ws_other123", Slug: "billing-fix", Role: "reviewer", Agent: "claude-code",
		State: core.StateIdle, CreatedAt: fixedNow.Add(-time.Hour), UpdatedAt: fixedNow,
	}
	h.be.Emit(protocol.TopicSession, other)
	h.pump()
	out := h.plain()
	if !strings.Contains(out, "workspace other1") || !strings.Contains(out, "billing-fix") {
		t.Fatalf("second workspace not grouped:\n%s", out)
	}
	ordered := h.m.orderedSessions()
	if ordered[len(ordered)-1].ID != "ses_other" {
		t.Fatalf("order = %v", ordered)
	}
	assertRect(t, h.screen(), 180, 48)
}

func TestClockRefreshesAges(t *testing.T) {
	h := newHarness(t, 180, 48)
	before := h.plain()
	h.m.now = func() time.Time { return fixedNow.Add(10 * time.Minute) }
	h.m.Update(clockMsg{})
	after := h.plain()
	if before == after || !strings.Contains(after, "23m") {
		t.Fatalf("ages did not refresh:\n%s", after)
	}
}
