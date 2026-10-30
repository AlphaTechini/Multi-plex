package diffview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

func plain(s string) string { return ansi.Strip(s) }

func reduced() *theme.Theme { return theme.DarkPlus().WithReducedMotion(true) }

// bigFileFake is a session with one file of many hunks, so scrolling runs
// deep, followed by another file.
func bigFileFake(hunks int) *fakeBackend {
	be := newFake()
	var raw strings.Builder
	var hs []string
	for i := 0; i < hunks; i++ {
		start := 1 + i*30
		hs = append(hs, hunkText(start, start, "func f"+itoa(i)+"()",
			" \ta := 1",
			" \tb := 2",
			" \tc := 3",
			"-\told"+itoa(i)+" := a + b",
			"+\tnew"+itoa(i)+" := a + b + c",
			" \td := 4",
			" \te := 5",
			" \tf := 6",
		))
	}
	raw.WriteString(fileDiff("a/big.go", hs...))
	raw.WriteString(fileDiff("b/next.go", hunkText(1, 1, "", " x", "-y", "+z", " w")))
	be.raw = raw.String()
	be.files["a/big.go"] = strings.Repeat("line\n", hunks*30+10)
	be.files["b/next.go"] = "x\nz\nw\n"
	return be
}

func TestScrollingNeverEntersNextFile(t *testing.T) {
	h := newHarness(t, reduced(), bigFileFake(40))
	h.render(112, 45)
	if got := h.current(); got != "a/big.go" {
		t.Fatalf("opened %q", got)
	}
	for i := 0; i < 400; i++ {
		h.key("ctrl+d")
	}
	for i := 0; i < 400; i++ {
		h.key("j")
	}
	h.key("G")
	for i := 0; i < 50; i++ {
		h.key("pgdown", "down", "end")
		h.send(wheel(0, 5, true))
	}
	if got := h.current(); got != "a/big.go" {
		t.Fatalf("scrolled into %q", got)
	}
	out := plain(h.render(112, 45))
	if !strings.Contains(out, "end of file") || !strings.Contains(out, "b/next.go") {
		t.Fatalf("end marker missing at the bottom:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	last := ""
	for _, l := range lines {
		if strings.Contains(l, "end of file") {
			last = l
		}
	}
	if !strings.Contains(last, "next") || !strings.Contains(last, "]") {
		t.Fatalf("end marker %q", last)
	}
	// The next-file key is the only way on.
	h.key("]")
	if got := h.current(); got != "b/next.go" {
		t.Fatalf("] opened %q", got)
	}
	// The last file's marker says so.
	h.key("G")
	out = plain(h.render(112, 45))
	if !strings.Contains(out, "last of 2 files") {
		t.Fatalf("last file marker missing:\n%s", out)
	}
	h.key("]", "]")
	if got := h.current(); got != "b/next.go" {
		t.Fatalf("] wrapped to %q", got)
	}
	h.key("[", "[", "[")
	if got := h.current(); got != "a/big.go" {
		t.Fatalf("[ wrapped to %q", got)
	}
}

func TestEndMarkerNamesNextFile(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.key("G")
	out := plain(h.render(112, 45))
	if !strings.Contains(out, "end of file  ·  next  src/auth/session_test.go") {
		t.Fatalf("marker:\n%s", out)
	}
	if h.m.fv.top != h.m.maxTop() {
		t.Fatalf("top %d, max %d", h.m.fv.top, h.m.maxTop())
	}
}

func TestOneFileAtATime(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	out := plain(h.render(112, 45))
	for _, other := range []string{"docs/auth.md  ", "func (c *Client) Do"} {
		if strings.Contains(out, "// content of") {
			t.Fatalf("another file's content on screen: %q", other)
		}
	}
	if !strings.Contains(out, "4 of 7") || !strings.Contains(out, "session.go") {
		t.Fatalf("banner:\n%s", out)
	}
	// Files in rail order.
	want := []string{"docs/auth.md", "src/api/client.go", "src/auth/cache.go", "src/auth/session.go", "src/auth/session_test.go", "src/auth/token.go", "src/worker/refresh.go"}
	for i, e := range h.m.files {
		if e.path != want[i] {
			t.Fatalf("rail order %d: %s, want %s", i, e.path, want[i])
		}
	}
}

func TestHunkNavigation(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.key("g")
	var seen []int
	for i := 0; i < 10; i++ {
		before := h.m.fv.cursor
		h.key("n")
		if h.m.fv.cursor == before {
			break
		}
		_, r := h.m.fv.units[h.m.fv.cursor].lines(&h.m.fv.e.file)
		l, _ := h.m.fv.units[h.m.fv.cursor].lines(&h.m.fv.e.file)
		if l != nil && l.Kind != 0 {
			seen = append(seen, l.OldNo)
		} else if r != nil {
			seen = append(seen, r.NewNo)
		}
	}
	// Changes: 41, 44, 46, 48 (add only), 97 (add run).
	want := []int{41, 44, 46, 48, 97}
	if len(seen) != len(want) {
		t.Fatalf("change starts %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("change starts %v, want %v", seen, want)
		}
	}
	h.key("N")
	if _, r := h.m.fv.units[h.m.fv.cursor].lines(&h.m.fv.e.file); r == nil || r.NewNo != 48 {
		t.Fatalf("N did not go back to 48")
	}
}

func TestCursorAndSelectionKeys(t *testing.T) {
	h := mockupHarness(t, reduced(), 180)
	h.render(112, 45)
	h.key("g")
	h.key("j", "j")
	if h.m.fv.cursor != 2 {
		t.Fatalf("cursor %d", h.m.fv.cursor)
	}
	h.key("J", "J")
	if h.m.fv.sel != 2 || h.m.fv.cursor != 4 {
		t.Fatalf("sel %d cursor %d", h.m.fv.sel, h.m.fv.cursor)
	}
	h.key("esc")
	if h.m.fv.sel != -1 {
		t.Fatalf("esc kept the selection")
	}
	h.key("V", "j", "j")
	if h.m.fv.sel < 0 || h.m.fv.cursor-h.m.fv.sel != 2 {
		t.Fatalf("visual: sel %d cursor %d", h.m.fv.sel, h.m.fv.cursor)
	}
	h.key("esc")
	h.key("G")
	if h.m.fv.cursor != len(h.m.fv.units)-1 {
		t.Fatalf("G: cursor %d of %d", h.m.fv.cursor, len(h.m.fv.units))
	}
}

func TestRailHiddenBelowThreshold(t *testing.T) {
	h := mockupHarness(t, reduced(), 110)
	narrow := plain(h.render(99, 30))
	if strings.Contains(narrow, "FILES") {
		t.Fatalf("rail shown at 99 columns")
	}
	if !strings.Contains(narrow, "f files") || !strings.Contains(narrow, "4 of 7") {
		t.Fatalf("narrow banner or hint row missing:\n%s", narrow)
	}
	wide := plain(h.render(112, 30))
	if !strings.Contains(wide, "FILES") || !strings.Contains(wide, "0/7 reviewed") {
		t.Fatalf("rail missing at 112 columns:\n%s", wide)
	}
	h.key("f")
	if strings.Contains(plain(h.render(112, 30)), "FILES") {
		t.Fatalf("f did not collapse the rail")
	}
	h.key("f")
	if !strings.Contains(plain(h.render(112, 30)), "FILES") {
		t.Fatalf("f did not bring the rail back")
	}
}

func TestFileOverlay(t *testing.T) {
	h := mockupHarness(t, reduced(), 110)
	h.render(99, 32)
	h.key("f")
	if !h.m.overlay {
		t.Fatal("f did not open the file list")
	}
	out := plain(h.render(99, 32))
	if !strings.Contains(out, "Files") || !strings.Contains(out, "auth.md") {
		t.Fatalf("overlay:\n%s", out)
	}
	h.key("g", "enter")
	if h.current() != "docs/auth.md" || h.m.overlay {
		t.Fatalf("overlay enter opened %q (overlay %v)", h.current(), h.m.overlay)
	}
	h.key("f", "esc")
	if h.m.overlay {
		t.Fatal("esc kept the overlay")
	}
}
