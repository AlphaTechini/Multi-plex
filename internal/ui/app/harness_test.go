package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/backend/fake"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// harness drives a Model without a terminal: it sends messages, runs the
// returned commands (bounded in time) and feeds their results back.
type harness struct {
	t  testing.TB
	m  *Model
	be *fake.Backend
}

func newHarness(t testing.TB, w, h int, opts ...fake.Option) *harness {
	t.Helper()
	be := fake.New(opts...)
	t.Cleanup(func() { be.Close() })
	th := theme.DarkPlus().WithReducedMotion(false)
	m := newModel(Options{Backend: be, WorkspaceDir: "/home/user/dev/multi-plex"}, th, demoProbe)
	m.noPump = true
	m.now = func() time.Time { return fixedNow }
	hn := &harness{t: t, m: m, be: be}
	hn.run(m.Init())
	hn.send(tea.WindowSizeMsg{Width: w, Height: h})
	hn.settle()
	return hn
}

var fixedNow = time.Now()

// send delivers a message and runs the commands it returns.
func (h *harness) send(msg tea.Msg) {
	h.t.Helper()
	_, cmd := h.m.Update(msg)
	h.runFor(cmd, 70*time.Millisecond)
}

// run executes cmd and feeds results back, for up to 250 ms.
func (h *harness) run(cmd tea.Cmd) { h.runFor(cmd, 150*time.Millisecond) }

func (h *harness) runFor(cmd tea.Cmd, limit time.Duration) {
	h.t.Helper()
	if cmd == nil {
		return
	}
	results := make(chan tea.Msg, 256)
	outstanding := 0
	var spawn func(c tea.Cmd)
	spawn = func(c tea.Cmd) {
		if c == nil {
			return
		}
		outstanding++
		go func() { results <- c() }()
	}
	spawn(cmd)
	deadline := time.After(limit)
	for outstanding > 0 {
		select {
		case msg := <-results:
			outstanding--
			switch r := msg.(type) {
			case nil:
			case tea.BatchMsg:
				for _, c := range r {
					spawn(c)
				}
			case tickMsg:
				// Animation ticks are driven by tests explicitly.
				h.m.tickArmed = false
			default:
				_, next := h.m.Update(msg)
				spawn(next)
			}
		case <-deadline:
			return
		}
	}
}

// settle lets pending async work (loads, snapshots, resizes) finish.
func (h *harness) settle() {
	h.t.Helper()
	h.run(func() tea.Msg { time.Sleep(70 * time.Millisecond); return nil })
	h.pump()
}

// pump feeds the fake's queued events to the model.
func (h *harness) pump() {
	h.t.Helper()
	var evs []backend.Event
	for {
		select {
		case ev, ok := <-h.be.Events():
			if !ok {
				goto done
			}
			evs = append(evs, ev)
			continue
		default:
		}
		break
	}
done:
	if len(evs) > 0 {
		h.send(eventsMsg{events: evs})
	}
}

func (h *harness) key(s string) {
	h.t.Helper()
	h.send(keyMsg(s))
}

// keys sends several keys, one by one.
func (h *harness) keys(ss ...string) {
	h.t.Helper()
	for _, s := range ss {
		h.key(s)
	}
}

// typeText types text rune by rune.
func (h *harness) typeText(s string) {
	h.t.Helper()
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// keyMsg builds a key press from a key name like "ctrl+\\", "enter", "p".
func keyMsg(s string) tea.KeyPressMsg {
	k := tea.KeyPressMsg{}
	mods := tea.KeyMod(0)
	for {
		switch {
		case strings.HasPrefix(s, "ctrl+") && len(s) > 5:
			mods |= tea.ModCtrl
			s = s[5:]
			continue
		case strings.HasPrefix(s, "alt+") && len(s) > 4:
			mods |= tea.ModAlt
			s = s[4:]
			continue
		case strings.HasPrefix(s, "shift+") && len(s) > 6:
			mods |= tea.ModShift
			s = s[6:]
			continue
		}
		break
	}
	k.Mod = mods
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "up": tea.KeyUp, "down": tea.KeyDown,
		"left": tea.KeyLeft, "right": tea.KeyRight, "backspace": tea.KeyBackspace, "space": tea.KeySpace,
		"home": tea.KeyHome, "end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
	}
	if c, ok := named[s]; ok {
		k.Code = c
		if s == "space" {
			k.Text = " "
		}
		return k
	}
	r := []rune(s)[0]
	k.Code = r
	if mods == 0 || mods == tea.ModShift {
		k.Text = s
	}
	return k
}

// screen returns the rendered screen as ANSI text.
func (h *harness) screen() string { return h.m.render() }

// plain returns the rendered screen without styling.
func (h *harness) plain() string { return stripANSI(h.m.render()) }

// shot writes the screen to $MPX_SHOTS_DIR/<name>.ansi when the variable is
// set (visual QA).
func (h *harness) shot(name string) {
	dir := os.Getenv("MPX_SHOTS_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(h.screen()+"\n"), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

var _ = colorprofile.TrueColor
