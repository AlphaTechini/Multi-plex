package diffview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// Visual QA: MPX_SHOTS=<dir> go test -run TestShots writes .ansi files of
// the Diff mode screens (the view inside the shell's chrome) for ansi2png.

// shotFrame is the region layout of docs/16 for the diff screen.
type shotFrame struct{ sidebar, main, agent int }

func shotLayout(w int) shotFrame {
	f := shotFrame{sidebar: 6}
	f.agent = max(60, (w*3+5)/10)
	if w-f.sidebar-f.agent < 72 {
		f.agent = 3
	}
	f.main = w - f.sidebar - f.agent
	return f
}

// shotScreen draws the view inside the shell's chrome at w × h.
func shotScreen(h *harness, w, hh int, toast bool) string {
	t := h.m.t
	f := shotLayout(w)
	body := hh - 1
	po := kit.PanelOptions{
		Width: f.main, Height: body, Flush: true, Focused: true,
		Header: kit.TabBar(t, kit.TabBarOptions{Tabs: []kit.Tab{{Label: "Overview", Badge: "3"}, {Label: "Diff", Badge: "7"}, {Label: "Code"}, {Label: "Specs"}, {Label: "Term"}}, Active: 1}),
		Meta:   "unified · 7 files",
	}
	iw, ih := kit.PanelInner(po)
	content := h.render(iw, ih)
	if toast {
		tt := kit.Toast(t, kit.ToastOptions{
			Width: 46, Tone: kit.ToneSuccess, Title: "Comment queued for auth-refactor",
			Body:  "2 queued · sent at the agent's next turn",
			Hints: []kit.Hint{{Keys: "ctrl+s", Label: "send now"}, {Keys: "u", Label: "undo"}},
		})
		if ih >= 40 {
			content = kit.Composite(content, tt, iw-kit.Width(tt)-2, ih-len(kit.Lines(tt))-1)
		}
	}
	po.Content = content
	side := kit.Panel(t, kit.PanelOptions{Width: f.sidebar, Height: body, Flush: true, Content: "\n  " + kit.StateDot(t, kit.StateWorking, 2) + "\n  3"})
	agentOpts := kit.PanelOptions{Width: f.agent, Height: body, Flush: true, Title: "auth-refactor", Meta: "claude"}
	if f.agent <= 3 {
		agentOpts.Title, agentOpts.Meta = "", ""
	}
	agent := kit.Panel(t, agentOpts)
	status := kit.StatusBar(t, kit.StatusBarOptions{Width: w, Left: []kit.Segment{{Text: "multi-plex", Bold: true}, {Text: "› auth-refactor"}}})
	return kit.JoinColumns(side, kit.Panel(t, po), agent) + "\n" + status
}

func writeShot(t *testing.T, name, s string) {
	dir := os.Getenv("MPX_SHOTS")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(s+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// mockupHarness is the harness at the state of the docs/16 mockup: file 3
// of 7 open, lines 44-46 selected and a comment being typed.
func mockupHarness(t *testing.T, th *theme.Theme, w int) *harness {
	h := newHarness(t, th, mockupFake())
	h.m.w, h.m.h = w, 45
	h.m.Render(w-2, 45)
	h.openPath("src/auth/session.go")
	return h
}

// openPath opens a file by path (like a click in the rail).
func (h *harness) openPath(path string) {
	h.t.Helper()
	h.send(view.SwitchModeMsg{Mode: view.ModeDiff, Path: path})
	if h.current() != path {
		h.t.Fatalf("could not open %s (have %s)", path, h.current())
	}
}

// selectNew selects new-side lines from..to.
func (h *harness) selectNew(from, to int) {
	h.t.Helper()
	fv := h.m.fv
	a := h.m.findUnitByNew(from)
	b := h.m.findUnitByNew(to)
	// Include the deleted line paired above the first added line.
	for a > 0 && fv.units[a-1].isChange(&fv.e.file) && !hasNew(h.m, a-1) {
		a--
	}
	fv.sel, fv.cursor = a, b
}

func hasNew(m *Model, ui int) bool {
	_, r := m.fv.units[ui].lines(&m.fv.e.file)
	return r != nil
}

func TestShots(t *testing.T) {
	if os.Getenv("MPX_SHOTS") == "" {
		t.Skip("set MPX_SHOTS to a directory to write ANSI screens")
	}
	dark := theme.DarkPlus()

	for _, sz := range [][2]int{{180, 48}, {110, 32}} {
		w, hh := sz[0], sz[1]
		h := mockupHarness(t, dark, w)
		h.selectNew(44, 46)
		h.key("c")
		h.typeText("Wrap with the user ID too, so the logs say whose refresh failed. And this path runs on every request: keep the cache lookup lock-free.")
		name := "diff-" + itoa(w) + "x" + itoa(hh)
		writeShot(t, name, shotScreen(h, w, hh, w >= 180))
	}

	// File switch: banner tinted, content fading in.
	for _, step := range []int{0, 3, 6} {
		h := mockupHarness(t, dark, 180)
		h.selectNew(44, 46)
		h.send(view.TickMsg{Frame: 1})
		h.key("[")
		h.key("]")
		h.clock.advance(time.Duration(step) * dark.Motion.FadeDuration / time.Duration(dark.Motion.FadeSteps))
		h.send(view.TickMsg{Frame: 2})
		writeShot(t, "diff-180x48-switch"+itoa(step), shotScreen(h, 180, 48, false))
	}

	// New hunk: the second hunk arrived with the last refresh.
	{
		be := mockupFake()
		full := be.raw
		be.raw = strings.Replace(full, sessionGoHunks()[1], "", 1)
		h := newHarness(t, dark, be)
		h.m.Render(112, 45)
		h.openPath("src/auth/session.go")
		be.raw = full
		h.refresh()
		h.clock.advance(300 * time.Millisecond)
		h.send(view.TickMsg{Frame: 1})
		writeShot(t, "diff-180x48-newhunk", shotScreen(h, 180, 48, false))
	}

	// 256 colours.
	{
		h := mockupHarness(t, dark.For(colorprofile.ANSI256), 180)
		h.selectNew(44, 46)
		writeShot(t, "diff-180x48-256", shotScreen(h, 180, 48, false))
	}

	// Side by side needs a main view of 200 columns: draw it alone.
	{
		h := mockupHarness(t, dark, 200)
		h.m.focused = true
		h.render(198, 46)
		h.key("s")
		h.selectNew(44, 46)
		h.writeComment("Wrap with the user ID too.")
		po := kit.PanelOptions{Width: 200, Height: 48, Flush: true, Focused: true, Meta: "split · 7 files",
			Header: kit.TabBar(dark, kit.TabBarOptions{Tabs: []kit.Tab{{Label: "Overview"}, {Label: "Diff", Badge: "7"}, {Label: "Code"}}, Active: 1})}
		po.Content = h.render(198, 46)
		writeShot(t, "diff-200x48-sbs", kit.Panel(dark, po))
	}

	// Empty and note states.
	{
		be := newFake()
		h := newHarness(t, dark, be)
		writeShot(t, "diff-180x48-empty", shotScreen(h, 180, 48, false))
		h2 := mockupHarness(t, dark, 110)
		h2.key("f")
		writeShot(t, "diff-110x32-files", shotScreen(h2, 110, 32, false))
	}
}
