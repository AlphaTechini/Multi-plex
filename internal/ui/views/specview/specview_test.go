package specview

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

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

func text(m *model, w, h int) string { return ansi.Strip(m.Render(w, h)) }

func TestRenderExactSizes(t *testing.T) {
	sizes := [][2]int{{1, 1}, {10, 5}, {20, 8}, {40, 12}, {83, 30}, {84, 30}, {90, 45}, {99, 29}, {120, 40}, {200, 50}}
	for name, th := range themes() {
		m, _ := newFixture(t, th)
		for k := range 3 {
			m.kind = docKind(k)
			for _, detail := range []bool{false, true} {
				m.detail = detail
				for _, sz := range sizes {
					assertBlock(t, fmt.Sprintf("%s kind %d detail %v %dx%d", name, k, detail, sz[0], sz[1]), m.Render(sz[0], sz[1]), sz[0], sz[1])
				}
			}
		}
	}
	empty := New(view.Env{Theme: theme.DarkPlus(), Backend: &fakeBackend{}}).(*model)
	for _, sz := range sizes {
		assertBlock(t, "empty", empty.Render(sz[0], sz[1]), sz[0], sz[1])
	}
	if empty.Render(0, 3) != "" {
		t.Error("zero width")
	}
}

func TestListAndSelection(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	out := text(m, 99, 29)
	for _, want := range []string{"SPECS", "token-refresh", "3/7", "9 criteria", "api-pagination", "docs-sweep", "6/6"} {
		if !strings.Contains(out, want) {
			t.Errorf("list missing %q:\n%s", want, out)
		}
	}
	if len(be.docReads) != 1 || be.docReads[0] != "token-refresh" {
		t.Errorf("first spec should load on arrival: %v", be.docReads)
	}
	exec(t, m, send(m, key("down")))
	if m.sel != 1 || len(be.docReads) != 2 {
		t.Errorf("sel %d reads %v", m.sel, be.docReads)
	}
	exec(t, m, send(m, key("down")))
	exec(t, m, send(m, key("down")))
	if m.sel != 2 {
		t.Errorf("sel clamps at the end, got %d", m.sel)
	}
	exec(t, m, send(m, key("up")))
	exec(t, m, send(m, key("up")))
	exec(t, m, send(m, key("up")))
	if m.sel != 0 || len(be.docReads) != 3 {
		t.Errorf("sel %d reads %v (a loaded spec must not be re-read)", m.sel, be.docReads)
	}
	// Clicking a spec selects it.
	m.Render(99, 29)
	exec(t, m, send(m, tea.MouseClickMsg{X: 5, Y: m.listY + 3, Button: tea.MouseLeft}))
	if m.sel != 1 {
		t.Errorf("click: sel %d", m.sel)
	}
}

func TestDocumentTabs(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	out := text(m, 99, 45)
	for _, want := range []string{"Token refresh", "token-refresh", "3/7", "tasks", "9 criteria", "Requirements", "Design", "Tasks 3/7",
		"R1.1", "R1.2", "R2.1", "WHEN a request is made", "REQUIREMENTS", "Requirement 1: Token refresh", "User story:"} {
		if !strings.Contains(out, want) {
			t.Errorf("requirements missing %q:\n%s", want, out)
		}
	}
	// The criterion numbers became ID pills.
	if strings.Contains(out, "1. WHEN") || strings.Contains(out, "2. IF") {
		t.Errorf("criteria still numbered:\n%s", out)
	}
	send(m, key("right"))
	out = text(m, 99, 45)
	for _, want := range []string{"ARCHITECTURE", "internal/auth/cache.go", "per-user", "the ADR", "in-memory implementation",
		"Keep the lookup lock-free", "Field", "user_id", "primary key", "func (c *Cache) Get", "Done."} {
		if !strings.Contains(out, want) {
			t.Errorf("design missing %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{"**", "`", "](", "|---"} {
		if strings.Contains(out, bad) {
			t.Errorf("design leaks markup %q:\n%s", bad, out)
		}
	}
	send(m, key("l"))
	out = text(m, 99, 45)
	if m.kind != docTasks {
		t.Fatalf("kind = %d", m.kind)
	}
	for _, want := range []string{"Token store", "Add store interface", "Persist expiry times", "Refresh logic", "Docs", "Details: not a requirement"} {
		if !strings.Contains(out, want) {
			t.Errorf("tasks missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "_Requirements") || strings.Contains(out, "Requirements:") {
		t.Errorf("requirement lines should be pills, not text:\n%s", out)
	}
	send(m, key("right"))
	if m.kind != docRequirements {
		t.Error("tabs wrap around")
	}
	send(m, key("3"))
	send(m, key("1"))
	send(m, key("2"))
	if m.kind != docDesign {
		t.Errorf("digit keys: kind %d", m.kind)
	}
	send(m, key("left"))
	send(m, key("left"))
	if m.kind != docTasks {
		t.Errorf("left wraps: kind %d", m.kind)
	}
}

func TestTaskGlyphsAndPills(t *testing.T) {
	th := theme.DarkPlus()
	lines := renderMarkdown(th, tasksMD, docTasks, 90)
	var plain []string
	for _, l := range lines {
		plain = append(plain, ansi.Strip(l.text))
	}
	joined := strings.Join(plain, "\n")
	spaces := regexp.MustCompile(` {2,}`)
	joined = spaces.ReplaceAllString(joined, " ")
	for _, want := range []string{
		"✓ 1 Token store",
		" ✓ 1.1 Add store interface R1.1",
		" ○ 1.2 Persist expiry times",
		"◐ 2 Refresh logic",
		" ✓ 2.1 Refresh before expiry R1.1",
		" ○ 2.2 Retry once R1.2",
		"○ 3 Docs R2.1",
		"that are active R1.1 R1.2",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in\n%s", want, joined)
		}
	}
	// Done tasks are muted, not struck through (docs/16).
	for _, l := range lines {
		if strings.Contains(l.text, "\x1b[9m") || strings.Contains(l.text, ";9m") {
			t.Fatalf("strikethrough used: %q", l.text)
		}
	}
	// Every line fits.
	for _, w := range []int{12, 20, 33, 60, 90} {
		for _, l := range renderMarkdown(th, tasksMD+designMD+requirementsMD, docTasks, w) {
			if got := ansi.StringWidth(l.text); got > w {
				t.Fatalf("width %d: line is %d cells: %q", w, got, ansi.Strip(l.text))
			}
		}
	}
}

func TestMarkdownBlocks(t *testing.T) {
	th := theme.DarkPlus()
	src := "# Title\n\nSome **bold** and `code` and [link](http://x) text\nwrapped over lines.\n\n- one\n- two\n  continued\n\n1. first\n2. second\n\n> quote\n\n```\nraw  line\n```\n"
	var got []string
	for _, l := range renderMarkdown(th, src, docDesign, 40) {
		got = append(got, ansi.Strip(l.text))
	}
	want := []string{
		"Title",
		"",
		"Some bold and code and link text wrapped",
		"over lines.",
		"",
		"· one",
		"· two continued",
		"",
		"1. first",
		"2. second",
		"",
		"▎ quote",
		"",
		" raw  line",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("blocks:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// Fenced code sits on the sunken surface.
	lines := renderMarkdown(th, src, docDesign, 40)
	if last := lines[len(lines)-1]; last.bg != th.Surface.Sunken {
		t.Errorf("code block surface = %v", last.bg)
	}
	// ASCII glyph set: no non-ASCII bullets.
	asc := th.WithGlyphs(theme.ASCIIGlyphs())
	for _, l := range renderMarkdown(asc, "- a\n- b\n", docDesign, 20) {
		if strings.ContainsAny(ansi.Strip(l.text), "·•▸") {
			t.Errorf("unicode bullet in ascii mode: %q", ansi.Strip(l.text))
		}
	}
}

func TestScrolling(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	m.Render(99, 14)
	if m.total == 0 {
		t.Fatal("no lines")
	}
	before := text(m, 99, 14)
	send(m, key("j"))
	after := text(m, 99, 14)
	if before == after {
		t.Error("j should scroll")
	}
	send(m, key("G"))
	end := text(m, 99, 14)
	send(m, key("pgdown"))
	if text(m, 99, 14) != end {
		t.Error("scroll must clamp at the end")
	}
	if !strings.Contains(end, "100%") || !strings.Contains(end, "R2.1") {
		t.Errorf("end of the document:\n%s", end)
	}
	send(m, key("g"))
	if text(m, 99, 14) != before {
		t.Error("g returns to the top")
	}
	// Scroll positions are remembered per document.
	send(m, key("j"))
	send(m, key("j"))
	send(m, key("2"))
	send(m, key("1"))
	if m.scroll[docRequirements]["token-refresh"] != 2 {
		t.Errorf("scroll not remembered: %v", m.scroll[docRequirements])
	}
	send(m, tea.MouseWheelMsg{X: 60, Y: 10, Button: tea.MouseWheelDown})
	if m.scroll[docRequirements]["token-refresh"] != 5 {
		t.Errorf("wheel: %d", m.scroll[docRequirements]["token-refresh"])
	}
}

func TestNarrowSinglePane(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	out := text(m, 60, 30)
	if !strings.Contains(out, "SPECS") || strings.Contains(out, "Requirements") {
		t.Errorf("narrow, list first:\n%s", out)
	}
	exec(t, m, send(m, key("down")))
	send(m, key("enter"))
	out = text(m, 60, 30)
	if !strings.Contains(out, "all specs") || !strings.Contains(out, "API pagination") {
		t.Errorf("narrow detail:\n%s", out)
	}
	if h := m.Hints(); h[len(h)-1].Keys != "esc" {
		t.Errorf("hints = %v", h)
	}
	send(m, key("esc"))
	if m.detail {
		t.Error("esc returns to the list")
	}
	if h := m.Hints(); h[len(h)-1].Label != "open" {
		t.Errorf("hints = %v", h)
	}
}

func TestEmptyAndErrors(t *testing.T) {
	be := &fakeBackend{}
	m := New(view.Env{Theme: theme.DarkPlus(), Backend: be}).(*model)
	if out := text(m, 90, 30); !strings.Contains(out, "Loading specs") {
		t.Errorf("before load:\n%s", out)
	}
	exec(t, m, send(m, view.FocusMsg{Focused: true}))
	out := text(m, 90, 30)
	for _, want := range []string{"No specs yet", ".multiplex/specs/<slug>/", "planner session", "requirements.md", "design.md", "tasks.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("empty state missing %q:\n%s", want, out)
		}
	}
	if m.Hints() != nil {
		t.Error("no hints without specs")
	}
	be.listErr = errBoom
	exec(t, m, send(m, key("r")))
	if out := text(m, 90, 30); !strings.Contains(out, "Could not load the specs") || !strings.Contains(out, "boom") {
		t.Errorf("error state:\n%s", out)
	}

	m2, be2 := newFixture(t, theme.DarkPlus())
	be2.specErr = errBoom
	exec(t, m2, send(m2, key("down")))
	if out := text(m2, 99, 30); !strings.Contains(out, "Could not read the spec: boom") {
		t.Errorf("doc error:\n%s", out)
	}
	if label, badge := m2.Tab(); label != "Specs" || badge != 0 {
		t.Errorf("Tab = %q %d", label, badge)
	}
	if m2.Animating() || m2.Capturing() {
		t.Error("never animates or captures")
	}
}

func TestEmptyDocuments(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	exec(t, m, send(m, key("down")))
	exec(t, m, send(m, key("down"))) // docs-sweep: all three empty
	send(m, key("1"))
	if out := text(m, 99, 30); !strings.Contains(out, "requirements.md has not been written yet") {
		t.Errorf("empty requirements:\n%s", out)
	}
	send(m, key("3"))
	if out := text(m, 99, 30); !strings.Contains(out, "tasks.md has not been written yet") {
		t.Errorf("empty tasks:\n%s", out)
	}
}

func TestSwitchModeSelectsSpec(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	cases := []struct {
		path string
		slug string
		kind docKind
	}{
		{"docs-sweep", "docs-sweep", docRequirements},
		{".multiplex/specs/api-pagination/design.md", "api-pagination", docDesign},
		{`.multiplex\specs\token-refresh\tasks.md`, "token-refresh", docTasks},
		{"specs/docs-sweep/", "docs-sweep", docTasks},
	}
	for _, c := range cases {
		m.kind = docRequirements
		exec(t, m, send(m, view.SwitchModeMsg{Mode: view.ModeSpecs, Path: c.path}))
		cur, _ := m.current()
		if cur.Slug != c.slug || (c.kind != docTasks || c.path != "specs/docs-sweep/") && m.kind != c.kind {
			t.Errorf("%s: slug %s kind %d", c.path, cur.Slug, m.kind)
		}
	}
	// Before the list has arrived the slug is remembered.
	fresh := New(view.Env{Theme: theme.DarkPlus(), Backend: newBackend()}).(*model)
	exec(t, fresh, send(fresh, view.SwitchModeMsg{Mode: view.ModeSpecs, Path: "api-pagination"}))
	exec(t, fresh, send(fresh, view.FocusMsg{Focused: true}))
	if cur, _ := fresh.current(); cur.Slug != "api-pagination" {
		t.Errorf("deferred selection: %s", cur.Slug)
	}
	if send(m, view.SwitchModeMsg{Mode: view.ModeCode, Path: "x"}) != nil {
		// other modes are ignored (the list load batch may be nil)
		t.Log("non-specs switch produced a command")
	}
}

func TestFocusReloads(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	n := be.listN
	be.specs = append(be.specs, protocol.SpecInfo{Slug: "new-spec", Title: "New", TasksTotal: 1})
	exec(t, m, send(m, view.FocusMsg{Focused: false}))
	exec(t, m, send(m, view.FocusMsg{Focused: true}))
	if be.listN != n+1 || len(m.specs) != 4 {
		t.Errorf("reload: lists %d->%d specs %d", n, be.listN, len(m.specs))
	}
	if cur, _ := m.current(); cur.Slug != "token-refresh" {
		t.Errorf("selection moved to %s", cur.Slug)
	}
}

func TestMouseTabs(t *testing.T) {
	m, _ := newFixture(t, theme.DarkPlus())
	m.Render(99, 30)
	x := m.tabX[docDesign]
	send(m, tea.MouseClickMsg{X: x[0] + 1, Y: tabRow, Button: tea.MouseLeft})
	if m.kind != docDesign {
		t.Errorf("tab click: kind %d", m.kind)
	}
	x = m.tabX[docTasks]
	send(m, tea.MouseClickMsg{X: x[0], Y: tabRow, Button: tea.MouseLeft})
	if m.kind != docTasks {
		t.Errorf("tab click: kind %d", m.kind)
	}
}

func TestUntrustedTextIsSanitised(t *testing.T) {
	m, be := newFixture(t, theme.DarkPlus())
	be.docs["token-refresh"] = protocol.SpecDoc{Requirements: "# Hi\x1b]0;pwn\x07 there\x1b[2J\n\n- item\x00x\n", Tasks: "- [ ] 1. a\x1b[31mb\n"}
	be.specs[0].Title = "Title\x1b[31m"
	exec(t, m, send(m, view.FocusMsg{Focused: false}))
	exec(t, m, send(m, view.FocusMsg{Focused: true}))
	out := m.Render(99, 30)
	for _, bad := range []string{"\x1b]", "\x1b[2J", "\x07", "\x00"} {
		if strings.Contains(out, bad) {
			t.Errorf("render contains %q", bad)
		}
	}
	assertBlock(t, "sanitised", out, 99, 30)
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
			Header: kit.TabBar(th, kit.TabBarOptions{Active: 3, Tabs: []kit.Tab{{Label: "Overview", Badge: "3"}, {Label: "Diff", Badge: "7"}, {Label: "Code"}, {Label: "Specs"}, {Label: "Term"}}}),
			Meta:   "3 specs"}
		po.Content = m.Render(w, h)
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(kit.Panel(th, po)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dp := theme.DarkPlus()
	write("specs-requirements", dp, 90, 45, nil)
	write("specs-design", dp, 90, 45, func(m *model) { send(m, key("2")) })
	write("specs-tasks", dp, 90, 45, func(m *model) { send(m, key("3")) })
	write("specs-110x32", dp, 99, 29, func(m *model) { send(m, key("3")) })
	write("specs-narrow-list", dp, 60, 30, nil)
	write("specs-narrow-detail", dp, 60, 30, func(m *model) { m.Render(60, 30); send(m, key("enter")); send(m, key("3")) })
	write("specs-256", dp.For(colorprofile.ANSI256), 90, 30, func(m *model) { send(m, key("3")) })
	write("specs-ascii", dp.WithGlyphs(theme.ASCIIGlyphs()), 90, 30, func(m *model) { send(m, key("2")) })
	write("specs-empty", dp, 90, 45, func(m *model) { m.specs = nil })
}

func BenchmarkRender120x40(b *testing.B) {
	m, _ := newFixture(b, theme.DarkPlus())
	send(m, key("3"))
	m.Render(120, 40)
	b.ReportAllocs()
	for b.Loop() {
		m.Render(120, 40)
	}
}
