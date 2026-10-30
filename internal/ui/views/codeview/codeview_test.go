package codeview

import (
	"encoding/json"
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
	sizes := [][2]int{{1, 1}, {5, 2}, {12, 3}, {20, 6}, {40, 12}, {80, 24}, {99, 29}, {120, 40}, {200, 50}}
	for name, th := range themes() {
		m, _ := newFixture(t, th)
		states := map[string]func(){
			"nothing open": func() {},
			"file": func() {
				openFile(t, m, "src/auth/session.go", 30)
				m.fresh = freshLines{first: 28, last: 31, at: epoch, on: true}
			},
			"follow": func() { m.follow = true },
			"paused": func() { m.paused = true },
			"picker": func() { m.picker = newPicker(th, m.changes) },
		}
		for _, st := range []string{"nothing open", "file", "follow", "paused", "picker"} {
			states[st]()
			for _, sz := range sizes {
				assertBlock(t, fmt.Sprintf("%s/%s %dx%d", name, st, sz[0], sz[1]), m.Render(sz[0], sz[1]), sz[0], sz[1])
			}
		}
		for _, p := range []string{"~/.claude/CLAUDE.md", "logo.png", "missing.go"} {
			m.picker = nil
			if p == "logo.png" {
				m.be.(*fakeBackend).files[p] = "\x89PNG"
			}
			openFile(t, m, p, 0)
			for _, sz := range sizes {
				assertBlock(t, fmt.Sprintf("%s/%s %dx%d", name, p, sz[0], sz[1]), m.Render(sz[0], sz[1]), sz[0], sz[1])
			}
		}
	}
	m, _ := newFixture(t, theme.DarkPlus())
	if m.Render(0, 5) != "" || m.Render(5, 0) != "" {
		t.Error("zero size must be empty")
	}
	m.Update(view.SessionMsg{})
	assertBlock(t, "no session", m.Render(80, 24), 80, 24)
}

func TestOpenAtLineAndBanner(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	openFile(t, m, "src/auth/session.go", 40)
	if m.cursor != 39 {
		t.Errorf("cursor = %d, want 39", m.cursor)
	}
	out := ansi.Strip(m.Render(100, 20))
	for _, want := range []string{"src/auth/", "session.go", "3 of 7", "modified", "+24", "Ln 40 of", "40  "} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if len(be.reads) != 1 || be.reads[0] != "src/auth/session.go" {
		t.Errorf("reads = %v", be.reads)
	}
	// The cursor line is on screen after a Render.
	lines := plain(m.Render(100, 20))
	found := false
	for _, l := range lines[headerRows:] {
		if strings.Contains(l, " 40  ") {
			found = true
		}
	}
	if !found {
		t.Errorf("line 40 not visible:\n%s", strings.Join(lines, "\n"))
	}
	// Paths with backslashes or ./ are normalised.
	openFile(t, m, `.\src\auth\cache.go`, 0)
	if m.path != "src/auth/cache.go" {
		t.Errorf("path = %q", m.path)
	}
	// Mode switches meant for other modes are ignored.
	if send(m, view.SwitchModeMsg{Mode: view.ModeDiff, Path: "x"}) != nil {
		t.Error("foreign mode switch handled")
	}
}

func TestScrollKeys(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	openFile(t, m, "big.go", 0)
	m.Render(100, 22) // 20 rows
	n := m.lines()
	if n < 700 {
		t.Fatalf("lines = %d", n)
	}
	steps := []struct {
		k      string
		cursor int
	}{{"j", 1}, {"down", 2}, {"k", 1}, {"ctrl+d", 11}, {"ctrl+u", 1}, {"pgdown", 21}, {"pgup", 1}, {"G", n - 1}, {"g", 0}, {"k", 0}}
	for _, s := range steps {
		send(m, key(s.k))
		m.Render(100, 22)
		if m.cursor != s.cursor {
			t.Fatalf("after %q cursor = %d, want %d", s.k, m.cursor, s.cursor)
		}
		if m.cursor < m.top || m.cursor >= m.top+20 {
			t.Fatalf("after %q cursor %d outside viewport [%d,%d)", s.k, m.cursor, m.top, m.top+20)
		}
	}
	send(m, key("G"))
	m.Render(100, 22)
	if m.top != n-20 {
		t.Errorf("G: top = %d, want %d", m.top, n-20)
	}
	send(m, tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if m.top != n-23 {
		t.Errorf("wheel up: top = %d", m.top)
	}
	if m.cursor >= m.top+20 {
		t.Error("cursor dragged along by the wheel")
	}
}

func TestHighlightIsAsyncAndWindowed(t *testing.T) {
	th := theme.DarkPlus()
	m, _ := newFixture(t, th)
	cmd := send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: "big.go", Line: 0})
	// Run only the read; keep the highlighting commands back.
	msg := cmd()
	cmd2 := send(m, msg)
	if cmd2 == nil {
		t.Fatal("opening a file must request highlighting")
	}
	before := m.Render(100, 22)
	if strings.Contains(before, "38;2;86;156;214") {
		t.Error("keyword colour present before any chunk arrived: highlighting ran on the render path")
	}
	exec(t, m, cmd2)
	after := m.Render(100, 22)
	if !strings.Contains(after, "38;2;86;156;214") {
		t.Error("keyword colour missing after the chunks arrived")
	}
	if before == after {
		t.Error("highlighting should change the output")
	}
	assertBlock(t, "highlighted", after, 100, 22)
}

func TestFollowMode(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	// Turn follow on: the daemon is told, the view starts waiting.
	exec(t, m, send(m, key("f")))
	if !m.follow || len(be.follows) != 1 || !be.follows[0] {
		t.Fatalf("follow=%v calls=%v", m.follow, be.follows)
	}
	if out := ansi.Strip(m.Render(100, 20)); !strings.Contains(out, "Waiting for the next edit") || !strings.Contains(out, "following") {
		t.Errorf("waiting state:\n%s", out)
	}

	// An edit opens the file at the edited lines and highlights them.
	exec(t, m, send(m, edit("src/auth/session.go", 38, 44)))
	if m.path != "src/auth/session.go" || m.cursor != 37 {
		t.Fatalf("path %q cursor %d", m.path, m.cursor)
	}
	if !m.fresh.on || m.fresh.first != 37 || m.fresh.last != 43 {
		t.Errorf("fresh = %+v", m.fresh)
	}
	if !m.Animating() {
		t.Error("highlight should animate")
	}
	out := ansi.Strip(m.Render(100, 20))
	if !strings.Contains(out, "following") || strings.Contains(out, "paused") {
		t.Errorf("status row:\n%s", out)
	}
	if !strings.Contains(out, "L38–L44") {
		t.Errorf("edit range missing from the status row:\n%s", out)
	}
	if m.top > 37 || m.top+18 <= 43 {
		t.Errorf("edited range not on screen: top %d", m.top)
	}

	// Scrolling away pauses following; the indicator says how to resume.
	send(m, key("j"))
	out = ansi.Strip(m.Render(100, 20))
	if !m.paused || !strings.Contains(out, "following paused · f to resume") {
		t.Errorf("paused: %v\n%s", m.paused, out)
	}
	// A later edit elsewhere does not move the view while paused.
	exec(t, m, send(m, edit("src/auth/cache.go", 3, 4)))
	if m.path != "src/auth/session.go" {
		t.Errorf("paused view jumped to %s", m.path)
	}
	// An edit to the open file while paused refreshes it in place.
	reads := len(be.reads)
	cursor := m.cursor
	exec(t, m, send(m, edit("src/auth/session.go", 10, 11)))
	if len(be.reads) != reads+1 || m.cursor != cursor {
		t.Errorf("paused refresh: reads %d->%d cursor %d->%d", reads, len(be.reads), cursor, m.cursor)
	}
	// f resumes and jumps to the newest edit (cache.go was newest? no: session.go L10).
	exec(t, m, send(m, key("f")))
	if m.paused || !m.follow || m.path != "src/auth/session.go" || m.cursor != 9 {
		t.Errorf("resume: paused=%v follow=%v path=%s cursor=%d", m.paused, m.follow, m.path, m.cursor)
	}
	// f again stops following.
	exec(t, m, send(m, key("f")))
	if m.follow || len(be.follows) != 2 || be.follows[1] {
		t.Errorf("stop: follow=%v calls=%v", m.follow, be.follows)
	}
	// With follow off, edits are ignored.
	reads = len(be.reads)
	exec(t, m, send(m, edit("src/auth/cache.go", 1, 2)))
	if len(be.reads) != reads || m.path != "src/auth/session.go" {
		t.Error("edit handled while follow is off")
	}
	// Edits of other sessions are ignored.
	m.follow = true
	m.paused = false
	b, _ := json.Marshal(core.EditEvent{SessionID: "other", Path: "src/auth/cache.go", StartLine: 1})
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicEdit, Data: b}))
	if m.path != "src/auth/session.go" {
		t.Error("another session's edit was followed")
	}
}

func TestFollowErrorRestoresState(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	be.setErr = errBoom
	out := exec(t, m, send(m, key("f")))
	if m.follow {
		t.Error("follow stays on after the daemon refused")
	}
	if len(out) != 1 {
		t.Fatalf("msgs = %v", out)
	}
	if toast, ok := out[0].(view.ToastMsg); !ok || toast.Tone != kit.ToneDanger {
		t.Errorf("toast = %+v", out[0])
	}
}

func TestSessionEventSyncsFollow(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	s := core.Session{ID: "s1", Follow: true}
	b, _ := json.Marshal(s)
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicSession, Data: b}))
	if !m.follow {
		t.Error("daemon follow flag not applied")
	}
}

func TestFollowNeverTypesIntoAgent(t *testing.T) {
	// fakeBackend leaves Message, Interrupt and PaneInput on a nil interface:
	// any call would panic. Drive every interaction that could matter.
	m, _ := newFixture(t, theme.DarkPlus())
	exec(t, m, send(m, key("f")))
	exec(t, m, send(m, edit("src/auth/session.go", 5, 6)))
	for _, k := range []string{"j", "k", "ctrl+d", "ctrl+u", "G", "g", "e", "p", "esc", "]", "["} {
		exec(t, m, send(m, key(k)))
	}
	send(m, tea.MouseClickMsg{X: 10, Y: 5, Button: tea.MouseLeft})
	send(m, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	exec(t, m, send(m, key("f")))
}

func TestPicker(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	send(m, key("p"))
	if !m.Capturing() {
		t.Fatal("picker should capture keys")
	}
	if len(m.picker.items) != 7 {
		t.Fatalf("items = %d, want all 7 changed files", len(m.picker.items))
	}
	out := ansi.Strip(m.Render(100, 30))
	for _, want := range []string{"src/auth/session.go", "docs/auth.md", "esc close"} {
		if !strings.Contains(out, want) {
			t.Errorf("picker missing %q:\n%s", want, out)
		}
	}
	typeText(t, m, "sessgo")
	if len(m.picker.items) == 0 || m.picker.items[0].path != "src/auth/session.go" {
		t.Fatalf("fuzzy order: %+v", m.picker.items)
	}
	exec(t, m, send(m, key("enter")))
	if m.Capturing() || m.path != "src/auth/session.go" {
		t.Errorf("enter: capturing=%v path=%q", m.Capturing(), m.path)
	}
	// Any typed path is offered.
	send(m, key("ctrl+p"))
	typeText(t, m, "AGENTS.md")
	last := m.picker.items[len(m.picker.items)-1]
	if last.path != "AGENTS.md" || last.change != nil {
		t.Fatalf("typed path item = %+v", last)
	}
	exec(t, m, send(m, key("enter")))
	if m.path != "AGENTS.md" {
		t.Errorf("typed path: %q", m.path)
	}
	// Navigation and esc.
	send(m, key("p"))
	send(m, key("down"))
	send(m, key("down"))
	if m.picker.sel != 2 {
		t.Errorf("sel = %d", m.picker.sel)
	}
	send(m, key("up"))
	send(m, key("esc"))
	if m.Capturing() {
		t.Error("esc should close")
	}
	n := len(be.reads)
	send(m, key("p"))
	send(m, key("enter")) // opens the first changed file
	_ = n
	assertBlock(t, "after", m.Render(100, 30), 100, 30)
}

func TestPickerKeysAreText(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	send(m, key("p"))
	typeText(t, m, "fjkg")
	if m.picker.in.Value() != "fjkg" || m.follow {
		t.Errorf("value %q follow %v: bound keys must be text in the picker", m.picker.in.Value(), m.follow)
	}
}

func TestGlobalFileAndNotes(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	exec(t, m, send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: "~/.claude/CLAUDE.md"}))
	if len(be.reads) != 0 {
		t.Errorf("global file was read through the backend: %v", be.reads)
	}
	out := ansi.Strip(m.Render(100, 20))
	if !strings.Contains(out, "opens in your editor") || !strings.Contains(out, "~/.claude/CLAUDE.md") {
		t.Errorf("global note:\n%s", out)
	}
	toasts := exec(t, m, send(m, key("e")))
	if len(toasts) != 1 {
		t.Fatalf("toasts = %v", toasts)
	}
	be.files["logo.png"] = "x"
	exec(t, m, send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: "logo.png"}))
	if out := ansi.Strip(m.Render(100, 20)); !strings.Contains(out, "Binary file") {
		t.Errorf("binary note:\n%s", out)
	}
	exec(t, m, send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: "nope.go"}))
	if out := ansi.Strip(m.Render(100, 20)); !strings.Contains(out, "Could not open the file") || !strings.Contains(out, "file not found") {
		t.Errorf("error note:\n%s", out)
	}
	// Steering file inside the workspace reads normally.
	exec(t, m, send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: "AGENTS.md"}))
	if out := ansi.Strip(m.Render(100, 20)); !strings.Contains(out, "# Agents") {
		t.Errorf("AGENTS.md:\n%s", out)
	}
}

func TestEditorToast(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	if len(exec(t, m, send(m, key("e")))) != 0 {
		t.Error("toast with no file open")
	}
	openFile(t, m, "src/auth/session.go", 12)
	msgs := exec(t, m, send(m, key("e")))
	if len(msgs) != 1 {
		t.Fatalf("msgs = %v", msgs)
	}
	toast := msgs[0].(view.ToastMsg)
	if toast.Body != "src/auth/session.go:12" {
		t.Errorf("toast body %q", toast.Body)
	}
}

func TestStepChange(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	exec(t, m, send(m, key("]")))
	if m.path != "src/api/client.go" {
		t.Errorf("first ] -> %q", m.path)
	}
	m.be.(*fakeBackend).files["src/api/client.go"] = "package api\n"
	exec(t, m, send(m, key("[")))
	if m.path != "docs/auth.md" {
		t.Errorf("[ wraps to %q", m.path)
	}
}

func TestFreshDecay(t *testing.T) {
	th := theme.DarkPlus()
	m, _ := newFixture(t, th)
	now := epoch
	m.now = func() time.Time { return now }
	m.clock = now
	exec(t, m, send(m, key("f")))
	m.fresh.at = now
	exec(t, m, send(m, edit("src/auth/session.go", 30, 33)))
	m.fresh.at = now
	m.Render(100, 20)
	lines := m.freshLevel()
	if lines != 1 {
		t.Errorf("initial level = %v", lines)
	}
	full := m.Render(100, 20)
	now = now.Add(600 * time.Millisecond)
	send(m, view.TickMsg{Frame: 3})
	mid := m.freshLevel()
	if mid <= 0 || mid >= 1 {
		t.Errorf("mid-decay level = %v", mid)
	}
	if !m.Animating() {
		t.Error("should animate while decaying")
	}
	if full == m.Render(100, 20) {
		t.Error("highlight colour should change while decaying")
	}
	now = now.Add(2 * time.Second)
	send(m, view.TickMsg{Frame: 9})
	if m.Animating() || m.freshLevel() != 0 || m.fresh.on {
		t.Errorf("highlight should be gone: %+v level %v", m.fresh, m.freshLevel())
	}
	// Reduced motion: a static marker, no animation.
	r, _ := newFixture(t, th.WithReducedMotion(true))
	exec(t, r, send(r, key("f")))
	exec(t, r, send(r, edit("src/auth/session.go", 30, 33)))
	if r.Animating() {
		t.Error("reduced motion must not animate")
	}
	out := ansi.Strip(r.Render(100, 20))
	if !strings.Contains(out, "▎") {
		t.Errorf("static marker missing:\n%s", out)
	}
	send(r, key("j"))
	if strings.Contains(ansi.Strip(r.Render(100, 20)), "▎") && r.fresh.on {
		t.Error("static marker should end at the first interaction")
	}
}

func TestSanitisesFileContent(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	be.files["evil.txt"] = "plain\n\x1b]0;pwn\x07 title\x1b[2J\nTab\there\x00 end\n"
	openFile(t, m, "evil.txt", 0)
	out := m.Render(80, 12)
	for _, bad := range []string{"\x1b]", "\x1b[2J", "\x07", "\x00", "\t"} {
		if strings.Contains(out, bad) {
			t.Errorf("render contains %q", bad)
		}
	}
	assertBlock(t, "evil", out, 80, 12)
	if !strings.Contains(ansi.Strip(out), "Tab") || !strings.Contains(ansi.Strip(out), "here") {
		t.Error("content lost")
	}
}

func TestChangesReloadOnDiffChanged(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	be.changes = be.changes[:2]
	b, _ := json.Marshal(protocol.SessionRef{SessionID: "s1"})
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicDiffChanged, Data: b}))
	if len(m.changes) != 2 {
		t.Errorf("changes = %d", len(m.changes))
	}
	b, _ = json.Marshal(protocol.SessionRef{SessionID: "other"})
	be.changes = nil
	exec(t, m, send(m, view.EventMsg{Topic: protocol.TopicDiffChanged, Data: b}))
	if len(m.changes) != 2 {
		t.Error("another session's diff event reloaded the list")
	}
}

func TestSessionSwitchResets(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	openFile(t, m, "src/auth/session.go", 0)
	s := core.Session{ID: "s2"}
	exec(t, m, send(m, view.SessionMsg{Session: &s}))
	if m.path != "" || m.src != nil {
		t.Error("file from the previous session still open")
	}
	// A late file result from the old session is dropped.
	late := fileMsg{req: openReq{seq: 1, sess: "s1", path: "src/auth/session.go"}}
	send(m, late)
	if m.src != nil {
		t.Error("stale file applied")
	}
}

func TestMouse(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	openFile(t, m, "src/auth/session.go", 0)
	m.Render(100, 20)
	send(m, tea.MouseClickMsg{X: 20, Y: headerRows + 4, Button: tea.MouseLeft})
	if m.cursor != 4 {
		t.Errorf("click: cursor %d", m.cursor)
	}
	send(m, tea.MouseClickMsg{X: 20, Y: 0, Button: tea.MouseLeft})
	if !m.Capturing() {
		t.Error("clicking the banner opens the picker")
	}
}

func TestHintsAndTab(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	if l, b := m.Tab(); l != "Code" || b != 0 {
		t.Errorf("Tab = %q %d", l, b)
	}
	if h := m.Hints(); h[0].Label != "follow" {
		t.Errorf("hints = %v", h)
	}
	m.follow = true
	if h := m.Hints(); h[0].Label != "stop following" {
		t.Errorf("hints = %v", h)
	}
	m.paused = true
	if h := m.Hints(); h[0].Label != "resume following" {
		t.Errorf("hints = %v", h)
	}
	m.picker = newPicker(m.th, nil)
	if h := m.Hints(); h[0].Keys != "up down" {
		t.Errorf("picker hints = %v", h)
	}
}

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
			Header: kit.TabBar(th, kit.TabBarOptions{Active: 2, Tabs: []kit.Tab{{Label: "Overview", Badge: "3"}, {Label: "Diff", Badge: "7"}, {Label: "Code"}, {Label: "Specs"}, {Label: "Term"}}}),
			Meta:   "follow on"}
		po.Content = m.Render(w, h)
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(kit.Panel(th, po)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dp := theme.DarkPlus()
	write("code-file", dp, 90, 45, func(m *model) { openFile(t, m, "src/auth/session.go", 31) })
	write("code-follow", dp, 90, 45, func(m *model) {
		exec(t, m, send(m, key("f")))
		exec(t, m, send(m, edit("src/auth/session.go", 31, 37)))
	})
	write("code-follow-fade", dp, 90, 45, func(m *model) {
		exec(t, m, send(m, key("f")))
		exec(t, m, send(m, edit("src/auth/session.go", 31, 37)))
		m.clock = m.fresh.at.Add(500 * time.Millisecond)
	})
	write("code-paused", dp, 90, 45, func(m *model) {
		exec(t, m, send(m, key("f")))
		exec(t, m, send(m, edit("src/auth/session.go", 31, 37)))
		send(m, key("j"))
	})
	write("code-picker", dp, 90, 45, func(m *model) {
		openFile(t, m, "src/auth/session.go", 31)
		send(m, key("p"))
		typeText(t, m, "auth")
	})
	write("code-empty", dp, 90, 45, nil)
	write("code-global", dp, 90, 45, func(m *model) {
		exec(t, m, send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: "~/.claude/CLAUDE.md"}))
	})
	write("code-110x32", dp, 99, 29, func(m *model) { openFile(t, m, "src/auth/session.go", 31) })
	write("code-256", dp.For(colorprofile.ANSI256), 90, 30, func(m *model) { openFile(t, m, "src/auth/session.go", 31) })
	write("code-ascii", dp.WithGlyphs(theme.ASCIIGlyphs()), 90, 30, func(m *model) {
		exec(t, m, send(m, key("f")))
		openFile(t, m, "src/auth/session.go", 31)
	})
}

func BenchmarkRender120x40(b *testing.B) {
	m, _ := newFixture(b, theme.DarkPlus())
	exec(b, m, send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: "big.go", Line: 500}))
	m.Render(120, 40)
	b.ReportAllocs()
	for b.Loop() {
		m.Render(120, 40)
	}
}

func BenchmarkRenderPicker120x40(b *testing.B) {
	m, _ := newFixture(b, theme.DarkPlus())
	exec(b, m, send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: "big.go", Line: 500}))
	send(m, key("p"))
	m.Render(120, 40)
	b.ReportAllocs()
	for b.Loop() {
		m.Render(120, 40)
	}
}
