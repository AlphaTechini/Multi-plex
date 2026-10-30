package overview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

func themes() map[string]*theme.Theme {
	base := theme.DarkPlus()
	return map[string]*theme.Theme{
		"truecolor": base,
		"256":       base.For(colorprofile.ANSI256),
		"ascii":     base.WithGlyphs(theme.ASCIIGlyphs()),
		"reduced":   base.WithReducedMotion(true),
	}
}

func assertBlock(t *testing.T, name, s string, w, h int) {
	t.Helper()
	lines := strings.Split(s, "\n")
	if len(lines) != h {
		t.Errorf("%s: %d lines, want %d", name, len(lines), h)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != w {
			t.Errorf("%s: line %d is %d cells, want %d: %q", name, i, got, w, ansi.Strip(l))
			return
		}
	}
}

func TestRenderExactSizes(t *testing.T) {
	sizes := [][2]int{{1, 1}, {4, 3}, {10, 5}, {20, 10}, {40, 12}, {60, 20}, {90, 45}, {99, 29}, {120, 40}, {200, 50}}
	for name, th := range themes() {
		m, _ := newFixture(t, th)
		for _, sz := range sizes {
			assertBlock(t, fmt.Sprintf("%s %dx%d", name, sz[0], sz[1]), m.Render(sz[0], sz[1]), sz[0], sz[1])
		}
		m.in = newInput(th, core.NoteAsk, "n1", 1)
		for _, sz := range sizes {
			assertBlock(t, fmt.Sprintf("%s input %dx%d", name, sz[0], sz[1]), m.Render(sz[0], sz[1]), sz[0], sz[1])
		}
	}
	m, _ := newFixture(t, theme.DarkPlus())
	if got := m.Render(0, 10); got != "" {
		t.Errorf("zero width: %q", got)
	}
}

func TestEmptyState(t *testing.T) {
	be := &fakeBackend{}
	m := New(view.Env{Theme: theme.DarkPlus(), Backend: be}).(*model)
	out := m.Render(90, 30)
	assertBlock(t, "empty", out, 90, 30)
	if !strings.Contains(ansi.Strip(out), "No session selected") {
		t.Error("empty state missing")
	}
	if label, badge := m.Tab(); label != "Overview" || badge != 0 {
		t.Errorf("Tab = %q %d", label, badge)
	}
	if send(m, tea.KeyPressMsg{Code: 'd', Text: "d"}) != nil {
		t.Error("keys with no session must do nothing")
	}
}

func TestLoadAndTab(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	if be.loads != 1 {
		t.Errorf("loads = %d, want 1", be.loads)
	}
	if label, badge := m.Tab(); label != "Overview" || badge != 3 {
		t.Errorf("Tab = %q %d, want Overview 3", label, badge)
	}
	if got := m.cards[0].ID; got != "n1" {
		t.Errorf("newest card = %s, want n1", got)
	}
	if m.spec == nil {
		t.Fatal("fixture spec missing")
	}
}

func TestEventsReloadOnlyForThisSession(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	ev := func(topic string, v any) view.EventMsg {
		return view.EventMsg{Topic: topic, Data: mustJSON(v)}
	}
	cases := []struct {
		name  string
		msg   view.EventMsg
		loads int
	}{
		{"note other", ev(protocol.TopicNote, core.Note{SessionID: "other"}), 0},
		{"note", ev(protocol.TopicNote, core.Note{SessionID: "s1"}), 1},
		{"note dismissed other", ev(protocol.TopicNoteDismissed, core.Note{ID: "n1", SessionID: "other", Dismissed: true}), 0},
		{"summary", ev(protocol.TopicSummary, core.Summary{SessionID: "s1"}), 1},
		{"conflicts", ev(protocol.TopicConflicts, protocol.ConflictsEvent{SessionID: "s1"}), 1},
		{"session same state", ev(protocol.TopicSession, core.Session{ID: "s1", State: core.StateWorking}), 0},
		{"session new state", ev(protocol.TopicSession, core.Session{ID: "s1", State: core.StateBlocked}), 1},
		{"unrelated topic", ev(protocol.TopicDiffChanged, protocol.SessionRef{SessionID: "s1"}), 0},
	}
	for _, c := range cases {
		before := be.loads
		exec(t, m, send(m, c.msg))
		if got := be.loads - before; got != c.loads {
			t.Errorf("%s: %d loads, want %d", c.name, got, c.loads)
		}
	}
	if m.detail.Session.State != core.StateWorking && m.sess.State != core.StateBlocked {
		t.Error("session state not applied")
	}
}

func TestNoteDismissedEventHidesTheCard(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	// Dismissed from another client: the daemon has not removed it from the
	// next detail yet, but the event hides it at once.
	before := be.loads
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicNoteDismissed, Data: mustJSON(core.Note{ID: "n1", SessionID: "s1", Number: 1, Dismissed: true})}))
	if be.loads == before {
		t.Error("note.dismissed did not reload")
	}
	if len(m.cards) != 2 || m.cards[0].ID != "n2" {
		t.Errorf("cards after note.dismissed = %+v", m.cards)
	}
	if _, badge := m.Tab(); badge != 2 {
		t.Errorf("badge = %d, want 2", badge)
	}
}

func TestCardNumbersAreStable(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	// The agent's third and fourth cards, newest first on screen: they keep
	// the numbers #4 and #3 that steer messages refer to.
	be.detail.Notes = []core.Note{
		{ID: "a", SessionID: "s1", Number: 3, Title: "Third card", At: epoch.Add(-5 * time.Minute)},
		{ID: "b", SessionID: "s1", Number: 4, Title: "Fourth card", At: epoch.Add(-time.Minute)},
	}
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(core.Note{SessionID: "s1"})}))
	text := strings.Join(plain(m.Render(90, 45)), "\n")
	i4, i3 := strings.Index(text, "4 INFO"), strings.Index(text, "3 INFO")
	if i4 < 0 || i3 < 0 || i4 > i3 {
		t.Fatalf("want card 4 above card 3:\n%s", text)
	}
	// A digit selects by shown number, and steer reports it.
	send(m, key("3"))
	if m.sel != 1 {
		t.Errorf("digit 3 selected row %d, want 1", m.sel)
	}
	send(m, key("s"))
	if m.in == nil || m.in.number != 3 {
		t.Fatalf("steer box = %+v, want card 3", m.in)
	}
}

func TestCoalescesLoads(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	n := core.Note{SessionID: "s1"}
	first := send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(n)})
	// Two more events while the first load is in flight collapse into one.
	if c := send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(n)}); c != nil {
		t.Error("second load started while one is in flight")
	}
	send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(n)})
	before := be.loads
	exec(t, m, first)
	if got := be.loads - before; got != 2 {
		t.Errorf("loads = %d, want 2 (in flight + one coalesced)", got)
	}
}

func TestSessionSwitchDiscardsStaleLoad(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	stale := send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(core.Note{SessionID: "s1"})})
	other := core.Session{ID: "s9", Slug: "other", Task: "Other task", State: core.StateIdle}
	be.detail = protocol.SessionDetail{Session: other}
	exec(t, m, send(m, view.SessionMsg{Session: &other}))
	if len(m.cards) != 0 {
		t.Fatalf("cards from the previous session leaked: %d", len(m.cards))
	}
	exec(t, m, stale) // result for s1 arrives late
	if m.detail.Session.ID != "s9" {
		t.Errorf("stale load applied: session %s", m.detail.Session.ID)
	}
	exec(t, m, send(m, view.SessionMsg{}))
	if m.sess != nil || m.hasDetail {
		t.Error("nil session must clear the view")
	}
}

func TestCardNavigationAndNew(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	delete(m.seen, "n3")
	text := func() string { return strings.Join(plain(m.Render(90, 45)), "\n") }
	if !strings.Contains(text(), "3 new") {
		t.Error("all three cards should be new at first")
	}
	send(m, key("down"))
	if !strings.Contains(text(), "2 new") {
		t.Error("leaving card 1 marks it seen")
	}
	for _, k := range []string{"j", "down", "j"} {
		send(m, key(k))
	}
	if m.sel != 2 {
		t.Errorf("sel = %d, want 2 (clamped)", m.sel)
	}
	if !strings.Contains(text(), "1 new") {
		t.Errorf("want 1 new:\n%s", text())
	}
	send(m, key("k"))
	send(m, key("1"))
	if m.sel != 0 {
		t.Errorf("digit 1 -> sel %d", m.sel)
	}
	send(m, key("3"))
	if m.sel != 2 {
		t.Errorf("digit 3 -> sel %d", m.sel)
	}
	if !strings.Contains(text(), "3 cards") {
		t.Errorf("all seen: want the card count, got:\n%s", text())
	}
	send(m, key("g"))
	if m.sel != 0 {
		t.Errorf("g -> %d", m.sel)
	}
	send(m, key("G"))
	if m.sel != 2 {
		t.Errorf("G -> %d", m.sel)
	}
}

func TestDismiss(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	cmd := send(m, key("d"))
	if len(m.cards) != 2 {
		t.Fatalf("card not removed at once: %d", len(m.cards))
	}
	if label, badge := m.Tab(); label != "Overview" || badge != 2 {
		t.Errorf("badge %d", badge)
	}
	exec(t, m, cmd)
	if len(be.acts) != 1 || be.acts[0].Action != core.NoteDismiss || be.acts[0].NoteID != "n1" {
		t.Errorf("acts = %+v", be.acts)
	}
	// The daemon still lists it until it catches up; it must stay hidden.
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(core.Note{SessionID: "s1"})}))
	if len(m.cards) != 2 {
		t.Errorf("dismissed card came back: %d", len(m.cards))
	}
}

func TestDismissFailureToasts(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	be.actErr = errBoom
	out := exec(t, m, send(m, key("d")))
	if len(out) != 1 {
		t.Fatalf("msgs = %v", out)
	}
	toast, ok := out[0].(view.ToastMsg)
	if !ok || toast.Tone != kit.ToneDanger || !strings.Contains(toast.Body, "boom") {
		t.Errorf("toast = %+v", out[0])
	}
}

func TestAskSteerInterruptFlow(t *testing.T) {
	cases := []struct {
		key    string
		action core.NoteAction
	}{{"a", core.NoteAsk}, {"s", core.NoteSteer}, {"i", core.NoteInterruptSteer}}
	for _, c := range cases {
		m, be := newFixture(t, theme.DarkPlus())
		if m.Capturing() {
			t.Fatal("capturing before input")
		}
		send(m, key(c.key))
		if !m.Capturing() {
			t.Fatalf("%s: input not open", c.key)
		}
		out := m.Render(90, 45)
		assertBlock(t, "input", out, 90, 45)
		if !strings.Contains(ansi.Strip(out), cardLabel(c.action, 1)) {
			t.Errorf("%s: box label missing:\n%s", c.key, ansi.Strip(out))
		}
		// Empty enter does nothing; text then enter sends.
		send(m, key("enter"))
		if !m.Capturing() || len(be.acts) != 0 {
			t.Fatal("empty message must not send")
		}
		typeText(t, m, "use a global cache")
		toasts := exec(t, m, send(m, key("enter")))
		if m.Capturing() {
			t.Error("input should close after sending")
		}
		if len(be.acts) != 1 || be.acts[0].Action != c.action || be.acts[0].NoteID != "n1" || be.acts[0].Text != "use a global cache" {
			t.Errorf("%s: acts = %+v", c.key, be.acts)
		}
		if len(toasts) != 1 {
			t.Errorf("%s: toasts = %v", c.key, toasts)
		}
	}
	m, be := newFixture(t, theme.DarkPlus())
	send(m, key("a"))
	typeText(t, m, "nope")
	send(m, key("esc"))
	if m.Capturing() || len(be.acts) != 0 {
		t.Error("esc must cancel without sending")
	}
	// While the box is open, "d" is text, not dismiss.
	send(m, key("a"))
	typeText(t, m, "d")
	if len(m.cards) != 3 || m.in.ti.Value() != "d" {
		t.Error("keys must go to the input while it is open")
	}
}

func TestOpenRefAndFiles(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	m.Render(90, 45)
	msgs := exec(t, m, send(m, key("enter")))
	want := view.SwitchModeMsg{Mode: view.ModeCode, Path: "src/auth/cache.go", Line: 12}
	if len(msgs) != 1 || msgs[0] != want {
		t.Errorf("enter on card 1: %v, want %v", msgs, want)
	}
	// up from the first card moves into the file links.
	send(m, key("up"))
	if m.zone != zoneFiles {
		t.Fatal("up from card 1 should select the files")
	}
	out := m.Render(90, 45)
	assertBlock(t, "files", out, 90, 45)
	msgs = exec(t, m, send(m, key("enter")))
	if len(msgs) != 1 || msgs[0] != (view.SwitchModeMsg{Mode: view.ModeDiff, Path: "src/auth/session.go"}) {
		t.Errorf("enter on first file: %v", msgs)
	}
	send(m, key("right"))
	msgs = exec(t, m, send(m, key("enter")))
	if len(msgs) != 1 || msgs[0] != (view.SwitchModeMsg{Mode: view.ModeDiff, Path: "src/auth/cache.go"}) {
		t.Errorf("enter on second file: %v", msgs)
	}
	send(m, key("right")) // "+5 more"
	msgs = exec(t, m, send(m, key("enter")))
	if len(msgs) != 1 || msgs[0] != (view.SwitchModeMsg{Mode: view.ModeDiff}) {
		t.Errorf("enter on +N more: %v", msgs)
	}
	send(m, key("esc"))
	if m.zone != zoneCards {
		t.Error("esc returns to cards")
	}
}

func TestMouse(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	lines := plain(m.Render(90, 45))
	find := func(sub string) (x, y int) {
		for i, l := range lines {
			if j := strings.Index(l, sub); j >= 0 {
				return len([]rune(l[:j])), i
			}
		}
		t.Fatalf("%q not on screen", sub)
		return
	}
	click := func(x, y int) []tea.Msg {
		return exec(t, m, send(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}))
	}
	// A file link opens the diff.
	x, y := find("src/auth/cache.go  ")
	if msgs := click(x+2, y); len(msgs) != 1 || msgs[0] != (view.SwitchModeMsg{Mode: view.ModeDiff, Path: "src/auth/cache.go"}) {
		t.Errorf("file click: %v", msgs)
	}
	// The ref of card 2 opens the code at its line.
	x, y = find("src/auth/session.go:L38-L52")
	if msgs := click(x+3, y); len(msgs) != 1 || msgs[0] != (view.SwitchModeMsg{Mode: view.ModeCode, Path: "src/auth/session.go", Line: 38}) {
		t.Errorf("ref click: %v", msgs)
	}
	if m.sel != 1 {
		t.Errorf("clicking a ref selects its card, sel = %d", m.sel)
	}
	// Clicking the title of card 3 selects it; clicking "steer" opens the box.
	_, y = find("Assumed tokens expire")
	click(20, y)
	if m.sel != 2 {
		t.Errorf("card click: sel %d", m.sel)
	}
	lines = plain(m.Render(90, 45))
	x, y = findLast(lines, "s steer")
	click(x+2, y)
	if !m.Capturing() || m.in.action != core.NoteSteer {
		t.Errorf("action click did not open steer box")
	}
	send(m, key("esc"))
	// Wheel moves the selection.
	send(m, tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if m.sel != 1 {
		t.Errorf("wheel up: sel %d", m.sel)
	}
}

func findLast(lines []string, sub string) (x, y int) {
	for i := len(lines) - 1; i >= 0; i-- {
		if j := strings.Index(lines[i], sub); j >= 0 {
			return len([]rune(lines[i][:j])), i
		}
	}
	return -1, -1
}

func TestFallbackToLastMessage(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	be.detail.Summary = nil
	be.detail.LastMessage = "I refactored the auth module and ran the tests.\nAll green.\x1b[31m"
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicSummary, Data: mustJSON(core.Summary{SessionID: "s1"})}))
	out := strings.Join(plain(m.Render(90, 45)), "\n")
	for _, want := range []string{"Agent said", "I refactored the auth module", "no structured report yet"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "report 6 of 6") || strings.Contains(out, "[31m") {
		t.Error("fallback shows a report meta or leaked an escape")
	}
}

func TestUntrustedTextIsSanitised(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	be.detail.Notes[0].Title = "evil\x1b]0;pwn\x07 title\x1b[2J"
	be.detail.Summary.Done = []string{"done\x1b[31m red \x1b[0m\x00x"}
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(core.Note{SessionID: "s1"})}))
	out := m.Render(90, 45)
	for _, bad := range []string{"\x1b]", "\x1b[2J", "\x07", "\x00"} {
		if strings.Contains(out, bad) {
			t.Errorf("render contains %q", bad)
		}
	}
	assertBlock(t, "sanitised", out, 90, 45)
}

func TestTailGroups(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	be.detail.Checkpoints = 4
	be.detail.Approvals = 5
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(core.Note{SessionID: "s1"})}))
	m.spec = &protocol.SpecInfo{Slug: "token-refresh", TasksDone: 3, TasksTotal: 7, CriteriaTotal: 9}
	out := strings.Join(plain(m.Render(90, 60)), "\n")
	for _, want := range []string{"CHECKPOINTS", "4 checkpoints", "5 approvals in the last hour",
		"Approvals are frequent — consider making this action reversible or allow-listed",
		"Will conflict with api-pagination in src/api/client.go", "token-refresh", "3/7", "9 criteria"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	be.detail.Approvals = 2
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(core.Note{SessionID: "s1"})}))
	out = strings.Join(plain(m.Render(90, 60)), "\n")
	if strings.Contains(out, "Approvals are frequent") {
		t.Error("hint shown at 2 approvals/hour")
	}
	be.detail.Checkpoints, be.detail.Approvals = 0, 0
	be.detail.Conflicts = nil
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(core.Note{SessionID: "s1"})}))
	m.spec = nil
	out = strings.Join(plain(m.Render(90, 60)), "\n")
	for _, bad := range []string{"CHECKPOINTS", "CONFLICTS", "SPEC"} {
		if strings.Contains(out, bad) {
			t.Errorf("empty section %s shown", bad)
		}
	}
}

func TestMatchSpec(t *testing.T) {
	specs := []protocol.SpecInfo{{Slug: "token"}, {Slug: "token-refresh"}, {Slug: "billing"}}
	s := core.Session{Task: "Implement the Token-Refresh spec"}
	if got := matchSpec(specs, s); got == nil || got.Slug != "token-refresh" {
		t.Errorf("match = %+v", got)
	}
	if got := matchSpec(specs, core.Session{Task: "unrelated"}); got != nil {
		t.Errorf("match = %+v", got)
	}
}

func TestWindowKeepsSelectionVisible(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	for i := range 8 {
		be.detail.Notes = append(be.detail.Notes, core.Note{ID: fmt.Sprintf("x%d", i), SessionID: "s1", Number: 10 + i,
			At: epoch.Add(-time.Duration(20+i) * time.Minute), Title: fmt.Sprintf("Extra card %d", i), Body: "body"})
	}
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicNote, Data: mustJSON(core.Note{SessionID: "s1"})}))
	if len(m.cards) != 11 {
		t.Fatalf("cards = %d", len(m.cards))
	}
	for i := range len(m.cards) {
		send(m, key("down"))
		out := m.Render(99, 29)
		assertBlock(t, "window", out, 99, 29)
		sel := m.sel
		if !strings.Contains(ansi.Strip(out), m.cards[sel].Title) {
			t.Fatalf("step %d: selected card %q not visible:\n%s", i, m.cards[sel].Title, ansi.Strip(out))
		}
	}
	for range len(m.cards) {
		send(m, key("up"))
		out := m.Render(99, 29)
		if m.zone == zoneCards && !strings.Contains(ansi.Strip(out), m.cards[m.sel].Title) {
			t.Fatalf("selected card %q not visible going up", m.cards[m.sel].Title)
		}
	}
}

func TestAnimatingAndHints(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	if !m.Animating() {
		t.Error("working session should animate")
	}
	r, _ := newFixture(t, theme.DarkPlus().WithReducedMotion(true))
	if r.Animating() {
		t.Error("reduced motion must not animate")
	}
	if len(m.Hints()) == 0 {
		t.Error("no hints")
	}
	send(m, key("a"))
	if h := m.Hints(); h[0].Label != "send" {
		t.Errorf("input hints = %v", h)
	}
	m.in = nil
	m.hasDetail = false
	if m.Animating() {
		t.Error("no animation before the detail loads")
	}
}

func TestElapsedAgeParseRef(t *testing.T) {
	for _, c := range []struct {
		got, want string
	}{
		{elapsed(42e9), "42s"}, {elapsed(14 * 60e9), "14m"}, {elapsed(125 * 60e9), "2h 5m"},
		{age(10e9), "now"}, {age(2 * 60e9), "2m"}, {age(3 * 3600e9), "3h"}, {age(72 * 3600e9), "3d"},
		{lowerFirst("Moved the"), "moved the"}, {lowerFirst("API change"), "API change"}, {lowerFirst("tokenValid"), "tokenValid"},
	} {
		if c.got != c.want {
			t.Errorf("got %q want %q", c.got, c.want)
		}
	}
	for _, c := range []struct {
		in   string
		path string
		line int
	}{
		{"src/a.go:L38-L52", "src/a.go", 38}, {"src/a.go:12", "src/a.go", 12}, {"src/a.go:L8", "src/a.go", 8},
		{"src/a.go", "src/a.go", 0}, {"src/a.go:L39–L41", "src/a.go", 39}, {"weird:thing", "weird:thing", 0},
	} {
		p, l := parseRef(c.in)
		if p != c.path || l != c.line {
			t.Errorf("parseRef(%q) = %q,%d want %q,%d", c.in, p, l, c.path, c.line)
		}
	}
}

// TestShots writes ANSI renderings of the view inside its panel for visual QA
// when MPX_SHOTS_DIR is set (see the convert step in the QA notes).
func TestShots(t *testing.T) {
	dir := os.Getenv("MPX_SHOTS_DIR")
	if dir == "" {
		t.Skip("MPX_SHOTS_DIR not set")
	}
	write := func(name string, th *theme.Theme, w, h int, setup func(*model)) {
		m, _ := newFixture(t, th)
		if setup != nil {
			setup(m)
		}
		po := kit.PanelOptions{Width: w + 2, Height: h + 2, Focused: true, Flush: true,
			Header: kit.TabBar(th, kit.TabBarOptions{Tabs: []kit.Tab{{Label: "Overview", Badge: "3"}, {Label: "Diff", Badge: "7"}, {Label: "Code"}, {Label: "Specs"}, {Label: "Term"}}}),
			Meta:   "updated 32s ago"}
		po.Content = m.Render(w, h)
		out := kit.Panel(th, po)
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(out+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(strings.Join(plain(m.Render(w, h)), "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dp := theme.DarkPlus()
	write("overview-180x48", dp, 90, 45, nil)
	write("overview-110x32", dp, 99, 29, nil)
	write("overview-110x32-256", dp.For(colorprofile.ANSI256), 99, 29, nil)
	write("overview-ascii", dp.WithGlyphs(theme.ASCIIGlyphs()), 90, 45, nil)
	write("overview-ask", dp, 90, 45, func(m *model) {
		send(m, key("a"))
		typeText(t, m, "Prefer a global cache keyed by tenant")
	})
	write("overview-files", dp, 90, 45, func(m *model) {
		m.Render(90, 45)
		send(m, key("up"))
		send(m, key("right"))
	})
	write("overview-noreport", dp, 90, 45, func(m *model) {
		m.detail.Summary = nil
		m.detail.LastMessage = "Token cache is in place and the auth tests pass. Before I update the callers in api/ and worker/: should the cache be per-user or global? I raised it as card 1."
		m.detail.Checkpoints, m.detail.Approvals = 4, 5
	})
	write("overview-empty", dp, 90, 45, func(m *model) { m.sess = nil })
}

func BenchmarkRender120x40(b *testing.B) {
	m, _ := newFixture(b, theme.DarkPlus())
	m.Render(120, 40)
	b.ReportAllocs()
	for b.Loop() {
		m.Render(120, 40)
	}
}
