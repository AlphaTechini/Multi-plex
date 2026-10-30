package pane

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// stub implements just the calls a Pane makes.
type stub struct {
	backend.Backend
	mu      sync.Mutex
	input   []byte
	resizes [][2]int
	snap    protocol.PaneFrame
}

func (s *stub) PaneInput(_ context.Context, _ core.PaneID, d []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	time.Sleep(time.Millisecond) // slow daemon: input queues up
	s.input = append(s.input, d...)
	return nil
}

func (s *stub) PaneResize(_ context.Context, _ core.PaneID, c, r int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resizes = append(s.resizes, [2]int{c, r})
	return nil
}

func (s *stub) PaneSnapshot(context.Context, core.PaneID) (protocol.PaneFrame, error) {
	return s.snap, nil
}

func newPane(t *testing.T) (*Pane, *stub) {
	t.Helper()
	be := &stub{}
	p := New(Options{Theme: theme.DarkPlus(), Backend: be, Name: "agent"})
	p.id = "p1"
	return p, be
}

func frame(full bool, cols, rows int, changed ...protocol.Row) protocol.PaneFrame {
	return protocol.PaneFrame{PaneID: "p1", Cols: cols, Rows: rows, Full: full, Changed: changed, CursorVisible: true}
}

func TestApplyFullAndPartial(t *testing.T) {
	p, _ := newPane(t)
	p.Apply(frame(true, 10, 3, protocol.Row{Y: 0, Text: "aaa"}, protocol.Row{Y: 1, Text: "bbb"}))
	p.Apply(frame(false, 10, 3, protocol.Row{Y: 1, Text: "BBB"}))
	if p.Row(0) != "aaa" || p.Row(1) != "BBB" {
		t.Fatalf("rows = %q %q", p.Row(0), p.Row(1))
	}
	p.Apply(frame(true, 10, 3, protocol.Row{Y: 2, Text: "ccc"}))
	if p.Row(0) != "" || p.Row(2) != "ccc" {
		t.Fatalf("full frame must replace: %q %q", p.Row(0), p.Row(2))
	}
	other := frame(true, 10, 3, protocol.Row{Y: 0, Text: "zzz"})
	other.PaneID = "other"
	p.Apply(other)
	if p.Row(0) != "" {
		t.Fatal("frame for another pane applied")
	}
}

func TestRenderExactSize(t *testing.T) {
	p, _ := newPane(t)
	p.Apply(frame(true, 20, 4,
		protocol.Row{Y: 0, Text: "\x1b[31mred\x1b[0m text"},
		protocol.Row{Y: 1, Text: strings.Repeat("x", 30)},
	))
	for _, sz := range [][2]int{{20, 4}, {10, 2}, {40, 9}, {1, 1}, {7, 6}} {
		out := p.Render(sz[0], sz[1], true)
		lines := strings.Split(out, "\n")
		if len(lines) != sz[1] {
			t.Fatalf("%v: %d lines", sz, len(lines))
		}
		for i, l := range lines {
			if w := ansi.StringWidth(l); w != sz[0] {
				t.Fatalf("%v line %d width %d", sz, i, w)
			}
		}
	}
}

func TestCursor(t *testing.T) {
	p, _ := newPane(t)
	f := frame(true, 10, 2, protocol.Row{Y: 0, Text: "abc"})
	f.CursorX, f.CursorY = 1, 0
	p.Apply(f)
	if got := p.Render(10, 2, true); !strings.Contains(got, "\x1b[7mb\x1b[0m") {
		t.Fatalf("focused cursor missing: %q", got)
	}
	if got := p.Render(10, 2, false); strings.Contains(got, "\x1b[7m") {
		t.Fatalf("unfocused pane draws a cursor: %q", got)
	}
	f.CursorVisible = false
	p.Apply(f)
	if got := p.Render(10, 2, true); strings.Contains(got, "\x1b[7m") {
		t.Fatal("hidden cursor drawn")
	}
}

func TestExitedNotice(t *testing.T) {
	p, _ := newPane(t)
	p.Apply(frame(true, 40, 3))
	p.MarkExited(core.Pane{ID: "p1", Exited: true, ExitCode: 1})
	out := ansi.Strip(p.Render(40, 3, true))
	last := strings.Split(out, "\n")[2]
	if !strings.Contains(last, "agent exited (code 1)") || !strings.Contains(last, "r restart") {
		t.Fatalf("exit row = %q", last)
	}
	if p.Key(tea.KeyPressMsg{Code: 'a', Text: "a"}) {
		t.Fatal("exited pane must not take keys")
	}
}

func TestKeyOrder(t *testing.T) {
	p, be := newPane(t)
	want := "hello world, this is typed fast"
	for _, r := range want {
		p.Key(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		be.mu.Lock()
		got := string(be.input)
		be.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	be.mu.Lock()
	defer be.mu.Unlock()
	t.Fatalf("input = %q, want %q", be.input, want)
}

func TestPasteBracketed(t *testing.T) {
	p, be := newPane(t)
	p.Paste("a\nb")
	time.Sleep(50 * time.Millisecond)
	be.mu.Lock()
	defer be.mu.Unlock()
	if !bytes.HasPrefix(be.input, []byte("\x1b[200~")) || !bytes.HasSuffix(be.input, []byte("\x1b[201~")) {
		t.Fatalf("paste = %q", be.input)
	}
}

func TestResizeDebounce(t *testing.T) {
	p, be := newPane(t)
	var msgs []Msg
	for _, sz := range [][2]int{{80, 24}, {81, 24}, {82, 25}} {
		cmd := p.Resize(sz[0], sz[1])
		if cmd == nil {
			t.Fatal("size change must schedule a send")
		}
		msgs = append(msgs, cmd().(Msg))
	}
	// Only the last tick may result in a call.
	for _, m := range msgs[:2] {
		if c := p.Update(m); c != nil {
			t.Fatal("stale tick sent a resize")
		}
	}
	cmd := p.Update(msgs[2])
	if cmd == nil {
		t.Fatal("last tick must send")
	}
	cmd()
	if len(be.resizes) != 1 || be.resizes[0] != [2]int{82, 25} {
		t.Fatalf("resizes = %v", be.resizes)
	}
	if p.Resize(82, 25) != nil {
		t.Fatal("same size must not schedule")
	}
}

func TestSnapshot(t *testing.T) {
	be := &stub{snap: frame(true, 10, 2, protocol.Row{Y: 0, Text: "snap"})}
	p := New(Options{Theme: theme.DarkPlus(), Backend: be})
	cmd := p.Attach("p1")
	msg := cmd().(SnapshotMsg)
	p.Update(msg)
	if p.Row(0) != "snap" {
		t.Fatalf("row 0 = %q", p.Row(0))
	}
	if p.Attach("p1") != nil {
		t.Fatal("re-attach must be a no-op")
	}
	// A live full frame beats a late snapshot.
	p2 := New(Options{Theme: theme.DarkPlus(), Backend: be})
	cmd = p2.Attach("p1")
	p2.Apply(frame(true, 10, 2, protocol.Row{Y: 0, Text: "live"}))
	p2.Update(cmd())
	if p2.Row(0) != "live" {
		t.Fatalf("late snapshot overwrote live frame: %q", p2.Row(0))
	}
}

func BenchmarkRender(b *testing.B) {
	be := &stub{}
	p := New(Options{Theme: theme.DarkPlus(), Backend: be})
	p.id = "p1"
	var rows []protocol.Row
	for y := range 46 {
		rows = append(rows, protocol.Row{Y: y, Text: "\x1b[38;5;244m" + strings.Repeat("hello ", 9) + "\x1b[0m"})
	}
	p.Apply(frame(true, 56, 46, rows...))
	b.ResetTimer()
	for i := range b.N {
		p.version = uint64(i) // defeat the cache: measure a real redraw
		p.Render(56, 46, true)
	}
	_ = kit.Width
}
