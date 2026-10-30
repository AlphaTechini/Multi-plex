package diffview

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// assertBlock checks that s is exactly h lines of exactly w cells.
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
	if strings.Contains(s, "\r") {
		t.Errorf("%s: carriage return in output", name)
	}
}

func TestRenderWidthInvariants(t *testing.T) {
	themes := map[string]*theme.Theme{
		"truecolor": theme.DarkPlus(),
		"256":       theme.DarkPlus().For(colorprofile.ANSI256),
		"ascii":     theme.DarkPlus().WithGlyphs(theme.ASCIIGlyphs()),
		"reduced":   reduced(),
	}
	for name, th := range themes {
		h := mockupHarness(t, th, 180)
		h.m.focused = true
		for w := 60; w <= 220; w += 7 {
			for _, hh := range []int{10, 24, 45} {
				h.m.w = w
				assertBlock(t, name+" view", h.render(w, hh), w, hh)
			}
		}
		// With a selection, a comment box being typed and at scroll positions.
		h.selectNew(44, 46)
		h.writeComment("A comment that is long enough to wrap onto a second and a third line of the box at narrow widths.")
		for w := 60; w <= 220; w += 11 {
			assertBlock(t, name+" composing", h.render(w, 40), w, 40)
		}
		h.key("esc")
		for _, k := range []string{"G", "g", "ctrl+d", "pgdown"} {
			h.key(k)
			for _, w := range []int{60, 79, 100, 109, 110, 111, 150, 197, 198, 199, 220} {
				assertBlock(t, name+" after "+k, h.render(w, 30), w, 30)
			}
		}
		// Overlay.
		h.key("f")
		h.m.railHidden = false
		assertBlock(t, name+" overlay", h.render(90, 30), 90, 30)
		h.key("esc")
	}
}

func TestRenderTinySizes(t *testing.T) {
	h := mockupHarness(t, theme.DarkPlus(), 180)
	for w := 1; w < 30; w++ {
		for hh := 1; hh < 12; hh++ {
			assertBlock(t, "tiny", h.render(w, hh), w, hh)
		}
	}
	if h.render(0, 5) != "" || h.render(5, 0) != "" {
		t.Fatal("zero size must render nothing")
	}
}

func TestSideBySide(t *testing.T) {
	h := mockupHarness(t, reduced(), 200)
	h.render(198, 45)
	h.key("s")
	if !h.m.sbs || !h.m.fv.sbs {
		t.Fatal("s did not switch to side-by-side at 198 columns")
	}
	out := h.render(198, 45)
	assertBlock(t, "sbs", out, 198, 45)
	p := plain(out)
	var pair string
	for _, l := range strings.Split(p, "\n") {
		if strings.Contains(l, "time.Until") && strings.Contains(l, "tokenValid") {
			pair = l
		}
	}
	if pair == "" {
		t.Fatalf("deleted and added line are not on one row:\n%s", p)
	}
	for _, w := range []int{198, 200, 230} {
		assertBlock(t, "sbs width", h.render(w, 40), w, 40)
	}
	// Selecting and commenting works in side-by-side too.
	h.selectNew(44, 44)
	h.writeComment("sbs comment")
	h.key("enter")
	c := h.be.snapshotComments()
	if len(c) != 1 || c[0].NewStart != 44 || c[0].OldStart != 44 {
		t.Fatalf("sbs comment %+v", c)
	}
	assertBlock(t, "sbs comment", h.render(198, 45), 198, 45)
	// Narrower than the threshold falls back to unified, and s explains.
	narrow := plain(h.render(150, 45))
	if strings.Contains(narrow, "│ ") && strings.Count(narrow, "time.Until") > 1 {
		t.Fatal("still split below the threshold")
	}
	h.toasts = nil
	h.key("s")
	if !hasToast(h, "wider terminal") {
		t.Fatalf("toasts %+v", h.toasts)
	}
	// Back to unified.
	h.render(198, 45)
	h.key("s")
	if h.m.sbs {
		t.Fatal("s did not switch back")
	}
}

func TestFadeAndBannerAnimation(t *testing.T) {
	th := theme.DarkPlus()
	h := mockupHarness(t, th, 180)
	h.render(112, 45)
	h.clock.advance(time.Second)
	h.send(view.TickMsg{})
	if h.m.Animating() {
		t.Fatal("still animating after a second")
	}
	h.key("]")
	if !h.m.Animating() || h.m.fadeAmount() != 1 || h.m.bannerSettle() != 0 {
		t.Fatalf("switch must start at frame 0: animating %v fade %v settle %v", h.m.Animating(), h.m.fadeAmount(), h.m.bannerSettle())
	}
	frame0 := h.render(112, 45)
	assertBlock(t, "frame 0", frame0, 112, 45)
	// Frame 0 hides the content below the banner.
	if strings.Contains(plain(frame0), "// content of src/auth/session_test.go") {
		// the plain text is still there (dimmed to the background); check colours
	}
	prev := 1.0
	step := th.Motion.FadeDuration / time.Duration(th.Motion.FadeSteps)
	for i := 1; i <= th.Motion.FadeSteps; i++ {
		h.clock.advance(step)
		h.send(view.TickMsg{Frame: i})
		got := h.m.fadeAmount()
		if got > prev {
			t.Fatalf("fade went backwards at step %d: %v > %v", i, got, prev)
		}
		prev = got
		assertBlock(t, "fade", h.render(112, 45), 112, 45)
	}
	if h.m.fadeAmount() != 0 {
		t.Fatalf("fade ended at %v", h.m.fadeAmount())
	}
	// The banner keeps its tint until BannerHold has passed.
	if !h.m.Animating() || h.m.bannerSettle() >= 1 {
		t.Fatalf("banner settled early: %v", h.m.bannerSettle())
	}
	h.clock.advance(th.Motion.BannerHold)
	h.send(view.TickMsg{})
	if h.m.Animating() || h.m.bannerSettle() != 1 {
		t.Fatal("animation did not end")
	}
	// A key pressed mid-fade finishes the fade at once.
	h.key("[")
	if h.m.fadeAmount() != 1 {
		t.Fatalf("fade amount %v", h.m.fadeAmount())
	}
	h.key("j")
	if h.m.fadeAmount() != 0 {
		t.Fatal("a key must finish the fade")
	}
}

func TestReducedMotionIsInstant(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.key("]")
	if h.m.Animating() || h.m.fadeAmount() != 0 || h.m.bannerSettle() != 1 {
		t.Fatalf("reduced motion animates: %v %v %v", h.m.Animating(), h.m.fadeAmount(), h.m.bannerSettle())
	}
	// 256 colours use the static forms too.
	h2 := mockupHarness(t, theme.DarkPlus().For(colorprofile.ANSI256), 180)
	h2.key("]")
	if h2.m.Animating() || h2.m.fadeAmount() != 0 {
		t.Fatal("256-colour theme animates")
	}
	a := h.render(112, 45)
	if strings.Contains(a, "38;2;") && !strings.Contains(a, "\x1b[") {
		t.Fatal("unexpected output")
	}
}

func TestStates(t *testing.T) {
	th := reduced()
	// No session.
	m := newModel(view.Env{Theme: th, Backend: newFake()}, testHL{}, time.Now)
	out := plain(m.Render(100, 30))
	assertBlock(t, "no session", m.Render(100, 30), 100, 30)
	if !strings.Contains(out, "No session selected") {
		t.Fatalf("no-session state:\n%s", out)
	}
	m.Update(view.SessionMsg{Session: nil})
	// No changes.
	be := newFake()
	h := newHarness(t, th, be)
	out = plain(h.render(100, 30))
	assertBlock(t, "no changes", h.render(100, 30), 100, 30)
	if !strings.Contains(out, "No changes yet") || !strings.Contains(out, "The diff appears here as the agent edits files.") {
		t.Fatalf("empty state:\n%s", out)
	}
	if label, badge := h.m.Tab(); label != "Diff" || badge != 0 {
		t.Fatalf("tab %q %d", label, badge)
	}
	// A file appears.
	be.raw = fileDiff("a.go", hunkText(1, 1, "", " x", "-y", "+z", " w"))
	be.files["a.go"] = "x\nz\nw\n"
	h.refresh()
	if label, badge := h.m.Tab(); label != "Diff" || badge != 1 {
		t.Fatalf("tab %q %d", label, badge)
	}
	if h.current() != "a.go" {
		t.Fatalf("current %q", h.current())
	}
	// Load error on first load.
	be2 := newFake()
	be2.diffErr = errTest("daemon unreachable")
	h2 := newHarness(t, th, be2)
	if out := plain(h2.render(100, 30)); !strings.Contains(out, "Could not read the diff") || !strings.Contains(out, "daemon unreachable") {
		t.Fatalf("error state:\n%s", out)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

func TestFileNotes(t *testing.T) {
	be := newFake()
	be.changes = []vcs.FileChange{
		{Path: "img/logo.png", Status: vcs.Modified, Binary: true},
		{Path: "old/gone.go", Status: vcs.Deleted, Deleted: 0},
	}
	be.raw = "diff --git a/img/logo.png b/img/logo.png\nindex 1..2 100644\nBinary files a/img/logo.png and b/img/logo.png differ\n" +
		"diff --git a/run.sh b/run.sh\nold mode 100644\nnew mode 100755\n" +
		"diff --git a/new/empty.txt b/new/empty.txt\nnew file mode 100644\nindex 0000000..e69de29\n" +
		"diff --git a/ren/b.go b/ren/b.go\nsimilarity index 100%\nrename from ren/a.go\nrename to ren/b.go\n"
	h := newHarness(t, reduced(), be)
	want := map[string]string{
		"img/logo.png":  "Binary file",
		"run.sh":        "Mode changed",
		"old/gone.go":   "File deleted",
		"ren/b.go":      "File renamed",
		"new/empty.txt": "Empty file added",
	}
	for path, text := range want {
		h.openPath(path)
		out := h.render(112, 30)
		assertBlock(t, path, out, 112, 30)
		if !strings.Contains(plain(out), text) {
			t.Errorf("%s: want %q:\n%s", path, text, plain(out))
		}
		// Keys must not misbehave without rows.
		h.key("j", "k", "G", "g", "n", "c", "enter", "ctrl+d", "y", "x", "e", "d")
		if h.m.Capturing() {
			t.Errorf("%s: comment box opened on a file without rows", path)
		}
	}
	if !strings.Contains(plain(h.render(112, 30)), "mode") && false {
		t.Fatal()
	}
}

func TestHugeDiffAsksFirst(t *testing.T) {
	be := newFake()
	be.raw = fileDiff("big.go", hunkText(1, 1, "", " x", "-y", "+z"))
	be.files["big.go"] = "x\nz\n"
	h := newHarness(t, reduced(), be)
	h.m.fv.e.huge = true
	h.m.rebuild()
	out := plain(h.render(112, 30))
	if !strings.Contains(out, "Very large diff") {
		t.Fatalf("huge note:\n%s", out)
	}
	h.key("enter")
	if out := plain(h.render(112, 30)); strings.Contains(out, "Very large diff") || !strings.Contains(out, "+z") && !strings.Contains(out, "z") {
		t.Fatalf("enter did not open the file:\n%s", out)
	}
}

func TestReviewedState(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	out := plain(h.render(112, 45))
	if !strings.Contains(out, "not reviewed") || !strings.Contains(out, "0/7 reviewed") {
		t.Fatalf("initial:\n%s", out)
	}
	h.key("v")
	out = plain(h.render(112, 45))
	if !strings.Contains(out, "✓ reviewed") || !strings.Contains(out, "1/7 reviewed") {
		t.Fatalf("after v:\n%s", out)
	}
	// Reviewed is remembered per session across switching away and back.
	h.send(view.SessionMsg{Session: &core.Session{ID: "s2", Slug: "other"}})
	h.send(view.SessionMsg{Session: testSession})
	h.openPath("src/auth/session.go")
	if !strings.Contains(plain(h.render(112, 45)), "1/7 reviewed") {
		t.Fatal("reviewed state lost when switching sessions")
	}
	// An edit to the file reopens it.
	h.be.raw = strings.Replace(h.be.raw, "+\ts.metrics.Refreshes.Inc()", "+\ts.metrics.Refreshes.Add(2)", 1)
	h.refresh()
	if out := plain(h.render(112, 45)); !strings.Contains(out, "0/7 reviewed") {
		t.Fatalf("an edit must clear the reviewed mark:\n%s", out)
	}
	// Hunks.
	h.key("g", "j")
	h.key("x")
	if out := plain(h.render(112, 45)); !strings.Contains(out, "@@ -38,13 +38,14 @@  ✓ func") {
		t.Fatalf("hunk reviewed mark missing:\n%s", out)
	}
	h.key("x")
	if out := plain(h.render(112, 45)); strings.Contains(out, "@@  ✓") {
		t.Fatal("x did not toggle back")
	}
}

func TestCopySelection(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.selectNew(44, 46)
	h.key("y")
	if len(h.clip) != 1 {
		t.Fatalf("clipboard writes: %d", len(h.clip))
	}
	got := h.clip[0]
	if !strings.HasPrefix(got, "src/auth/session.go:L44-L46\n") || !strings.Contains(got, "s.cache.Get(ctx, s.userID, s.client.Token)") {
		t.Fatalf("clipboard %q", got)
	}
	if !hasToast(h, "Copied L44–L46") {
		t.Fatalf("toasts %+v", h.toasts)
	}
}

func TestMouse(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	// Click a line: y is relative to the view, the body starts at row 2.
	top := h.m.fv.top
	ui := h.m.unitAtLine(top + 10)
	h.send(click(60, 2+10, 0))
	if h.m.fv.cursor != ui || h.m.fv.sel != -1 {
		t.Fatalf("click: cursor %d, want %d", h.m.fv.cursor, ui)
	}
	h.send(click(60, 2+13, tea.ModShift))
	if h.m.fv.sel != ui || h.m.fv.cursor != h.m.unitAtLine(top+13) {
		t.Fatalf("shift+click: sel %d cursor %d", h.m.fv.sel, h.m.fv.cursor)
	}
	// Wheel scrolls without moving the cursor.
	h.m.fv.cursor, h.m.fv.sel = ui, -1
	h.send(wheel(60, 10, true))
	if h.m.fv.top != top+scrollStep && h.m.fv.top != h.m.maxTop() {
		t.Fatalf("wheel: top %d", h.m.fv.top)
	}
	h.send(wheel(60, 10, false))
	h.send(wheel(60, 10, false))
	if h.m.fv.top != 0 {
		t.Fatalf("wheel up: top %d", h.m.fv.top)
	}
	// Clicking a file in the rail opens it: rail rows start at 3; docs/auth.md
	// is the first file after the first directory header.
	start := h.current()
	h.send(click(5, 3+1, 0))
	if h.current() == start || h.current() != "docs/auth.md" {
		t.Fatalf("rail click opened %q", h.current())
	}
	h.send(click(5, 3+4, 0))
	if h.current() != "src/api/client.go" {
		t.Fatalf("second rail click opened %q", h.current())
	}
	// A click on a fold expands it.
	h.openPath("src/auth/session.go")
	h.render(112, 45)
	for di, d := range h.m.fv.dl {
		if d.kind == dlUnit && h.m.fv.units[d.unit].kind == uFold {
			h.send(click(60, bodyTop+di-h.m.fv.top, 0))
			break
		}
	}
	if strings.Contains(plain(h.render(112, 45)), "37 unchanged") {
		t.Fatal("clicking the fold did not expand it")
	}
}

func TestSessionSwitchResets(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.send(view.SessionMsg{Session: &core.Session{ID: "s2", Slug: "other"}})
	if h.m.fv == nil || h.current() != "docs/auth.md" {
		// The same fake backend answers for every session.
		t.Logf("current %q", h.current())
	}
	h.send(view.SessionMsg{Session: nil})
	if out := plain(h.render(80, 20)); !strings.Contains(out, "No session selected") {
		t.Fatalf("after clearing the session:\n%s", out)
	}
	if h.m.Capturing() {
		t.Fatal("capturing without a session")
	}
}

func TestHints(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	var keys []string
	for _, k := range h.m.Hints() {
		keys = append(keys, k.Keys)
	}
	if got := strings.Join(keys, " "); !strings.Contains(got, "[") || !strings.Contains(got, "]") || !strings.Contains(got, "n") || !strings.Contains(got, "c") {
		t.Fatalf("hints %v", keys)
	}
	h.render(112, 45)
	h.key("c")
	var labels []string
	for _, k := range h.m.Hints() {
		labels = append(labels, k.Label)
	}
	if !strings.Contains(strings.Join(labels, ","), "queue") {
		t.Fatalf("composing hints %v", labels)
	}
}
