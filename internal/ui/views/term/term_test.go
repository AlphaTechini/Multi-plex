package term

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
	"github.com/alphatechini/multi-plex/internal/ui/pane"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// host plays the shell for a view: it feeds commands' results back.
type host struct {
	t *testing.T
	v view.View
}

// do sends msg and settles the commands it returns, routing pane messages
// the way the shell does.
func (h *host) do(msg tea.Msg) {
	h.t.Helper()
	var next tea.Cmd
	h.v, next = h.v.Update(msg)
	h.settle(next)
}

func (h *host) settle(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		switch m := msg.(type) {
		case nil:
		case tea.BatchMsg:
			for _, c := range m {
				h.settle(c)
			}
		case pane.Msg:
			h.settle(m.Dest().Update(m))
		default:
			var next tea.Cmd
			h.v, next = h.v.Update(msg)
			h.settle(next)
		}
	case <-time.After(300 * time.Millisecond):
	}
}

func newHost(t *testing.T) (*host, *fake.Backend, core.Session) {
	t.Helper()
	be := fake.New()
	t.Cleanup(func() { be.Close() })
	ss, _ := be.Sessions(t.Context(), "")
	h := &host{t: t, v: New(view.Env{Theme: theme.DarkPlus(), Backend: be})}
	h.do(tea.WindowSizeMsg{Width: 60, Height: 10})
	return h, be, ss[0]
}

func (h *host) countCalls(be *fake.Backend, name string) int {
	n := 0
	for _, c := range be.Calls() {
		if c == name {
			n++
		}
	}
	return n
}

func TestOpensLazilyOncePerSession(t *testing.T) {
	h, be, s := newHost(t)
	if h.countCalls(be, "OpenShell") != 0 {
		t.Fatal("shell opened before the view was shown")
	}
	h.do(view.SessionMsg{Session: &s})
	if h.countCalls(be, "OpenShell") != 1 {
		t.Fatalf("OpenShell called %d times", h.countCalls(be, "OpenShell"))
	}
	h.do(view.SessionMsg{Session: &s})
	if h.countCalls(be, "OpenShell") != 1 {
		t.Fatal("shell opened twice for one session")
	}
	// Another session gets its own shell; going back reuses the first.
	other, _ := be.Sessions(t.Context(), "")
	h.do(view.SessionMsg{Session: &other[1]})
	h.do(view.SessionMsg{Session: &s})
	if h.countCalls(be, "OpenShell") != 2 {
		t.Fatalf("OpenShell called %d times for two sessions", h.countCalls(be, "OpenShell"))
	}
}

func TestCapturingFollowsFocus(t *testing.T) {
	h, _, _ := newHost(t)
	if h.v.Capturing() {
		t.Fatal("capturing while unfocused")
	}
	h.do(view.FocusMsg{Focused: true})
	if !h.v.Capturing() {
		t.Fatal("not capturing while focused")
	}
	if label, badge := h.v.Tab(); label != "Term" || badge != 0 {
		t.Fatalf("tab = %q %d", label, badge)
	}
	if h.v.Animating() {
		t.Fatal("term never animates")
	}
}

func TestRenderStates(t *testing.T) {
	h, _, s := newHost(t)
	if out := ansi.Strip(h.v.Render(60, 10)); !strings.Contains(out, "No session selected") {
		t.Fatalf("no-session render:\n%s", out)
	}
	var cmd tea.Cmd
	h.v, cmd = h.v.Update(view.SessionMsg{Session: &s}) // the open is still pending
	_ = cmd
	if out := ansi.Strip(h.v.Render(60, 10)); !strings.Contains(out, "Opening a shell") {
		t.Fatalf("opening render:\n%s", out)
	}
	for _, l := range strings.Split(h.v.Render(60, 10), "\n") {
		if ansi.StringWidth(l) != 60 {
			t.Fatalf("line width %d", ansi.StringWidth(l))
		}
	}
}

func frameEvent(t *testing.T, f protocol.PaneFrame) view.EventMsg {
	t.Helper()
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return view.EventMsg{Topic: protocol.TopicPaneFrame, Data: data}
}

func TestShellFramesAndTyping(t *testing.T) {
	h, be, s := newHost(t)
	h.do(view.SessionMsg{Session: &s})
	p := h.v.(*View).current()
	if p == nil {
		t.Fatal("no pane after open")
	}
	h.do(view.FocusMsg{Focused: true})
	if out := ansi.Strip(h.v.Render(60, 10)); !strings.Contains(out, "dev@multiplex") {
		t.Fatalf("the snapshot was not drawn:\n%s", out)
	}
	// Typed keys reach the daemon, in order, and its frames come back as events.
	for _, r := range "echo hi" {
		h.do(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	h.do(tea.KeyPressMsg{Code: tea.KeyEnter})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && h.countCalls(be, "PaneInput") == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	snap, _ := be.PaneSnapshot(t.Context(), p.ID())
	h.do(frameEvent(t, snap))
	if out := ansi.Strip(h.v.Render(60, 10)); !strings.Contains(out, "echo hi") {
		t.Fatalf("typed command missing:\n%s", out)
	}
	// Keys are ignored while unfocused.
	n := h.countCalls(be, "PaneInput")
	h.do(view.FocusMsg{Focused: false})
	h.do(tea.KeyPressMsg{Code: 'x', Text: "x"})
	time.Sleep(30 * time.Millisecond)
	if h.countCalls(be, "PaneInput") != n {
		t.Fatal("an unfocused view took input")
	}
	// The shell only sends pane events to views that want them.
	if w, ok := h.v.(interface{ WantsPaneFrames() bool }); !ok || !w.WantsPaneFrames() {
		t.Fatal("term must ask for pane frames")
	}
}

func TestExitedShellRestartsOnR(t *testing.T) {
	h, be, s := newHost(t)
	h.do(view.SessionMsg{Session: &s})
	h.do(view.FocusMsg{Focused: true})
	p := h.v.(*View).current()
	data, _ := json.Marshal(core.Pane{ID: p.ID(), Exited: true, ExitCode: 2})
	h.do(view.EventMsg{Topic: protocol.TopicPaneExited, Data: data})
	if out := ansi.Strip(h.v.Render(60, 10)); !strings.Contains(out, "shell exited (code 2)") {
		t.Fatalf("exit notice missing:\n%s", out)
	}
	h.do(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if h.countCalls(be, "OpenShell") != 2 {
		t.Fatalf("r must open a new shell, OpenShell called %d times", h.countCalls(be, "OpenShell"))
	}
}
