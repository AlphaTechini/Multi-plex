package term

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/pty"
	"github.com/alphatechini/multi-plex/internal/vt/charmvt"
)

// sinkRecorder collects frames and lets tests wait for conditions.
type sinkRecorder struct {
	mu     sync.Mutex
	frames []protocol.PaneFrame
	times  []time.Time
}

func (s *sinkRecorder) sink(f protocol.PaneFrame) {
	s.mu.Lock()
	s.frames = append(s.frames, f)
	s.times = append(s.times, time.Now())
	s.mu.Unlock()
}

func (s *sinkRecorder) all() []protocol.PaneFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]protocol.PaneFrame(nil), s.frames...)
}

// screen applies the recorded frames to a blank screen and returns plain rows.
func (s *sinkRecorder) screen() []string {
	var rows []string
	for _, f := range s.all() {
		if f.Full || len(rows) != f.Rows {
			rows = make([]string, f.Rows)
		}
		for _, r := range f.Changed {
			rows[r.Y] = strings.TrimRight(ansi.Strip(r.Text), " ")
		}
	}
	return rows
}

func (s *sinkRecorder) waitScreen(t *testing.T, cond func(rows []string) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond(s.screen()) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for screen; have %q", s.screen())
}

func startFake(t *testing.T, cols, rows int) (*Host, *fakeProc, *sinkRecorder) {
	t.Helper()
	fp := newFakeProc()
	rec := &sinkRecorder{}
	h, err := Start(context.Background(), fakeSpawner{fp}, charmvt.Factory,
		pty.Spec{Argv: []string{"fake"}, Cols: cols, Rows: rows}, "pane_test", rec.sink)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	// Let the initial full frame go out before the test emits anything.
	rec.waitScreen(t, func([]string) bool { return len(rec.all()) > 0 })
	return h, fp, rec
}

func TestFirstFrameIsFull(t *testing.T) {
	_, _, rec := startFake(t, 20, 4)
	f := rec.all()[0]
	if !f.Full || f.Cols != 20 || f.Rows != 4 || len(f.Changed) != 4 || f.PaneID != "pane_test" {
		t.Errorf("first frame = %+v", f)
	}
}

func TestOutputBecomesFrames(t *testing.T) {
	_, fp, rec := startFake(t, 20, 4)
	fp.emit("hello\r\n\x1b[31mred\x1b[0m")
	rec.waitScreen(t, func(r []string) bool { return len(r) == 4 && r[0] == "hello" && r[1] == "red" })

	frames := rec.all()
	last := frames[len(frames)-1]
	if last.Full {
		t.Error("an incremental update must not be a full frame")
	}
	for _, r := range last.Changed {
		if ansi.StringWidth(r.Text) != 20 {
			t.Errorf("row %d is %d wide, want 20", r.Y, ansi.StringWidth(r.Text))
		}
	}
	if last.CursorX != 3 || last.CursorY != 1 || !last.CursorVisible {
		t.Errorf("cursor = %d,%d visible=%v", last.CursorX, last.CursorY, last.CursorVisible)
	}
}

func TestOnlyChangedRowsAreSent(t *testing.T) {
	_, fp, rec := startFake(t, 20, 6)
	fp.emit("a\r\nb\r\nc")
	rec.waitScreen(t, func(r []string) bool { return len(r) == 6 && r[2] == "c" })
	before := len(rec.all())

	fp.emit("\x1b[5;1Hxyz")
	rec.waitScreen(t, func(r []string) bool { return r[4] == "xyz" })
	frames := rec.all()[before:]
	var rows []int
	for _, f := range frames {
		for _, r := range f.Changed {
			rows = append(rows, r.Y)
		}
	}
	if len(rows) != 1 || rows[0] != 4 {
		t.Errorf("changed rows = %v, want [4]", rows)
	}
}

func TestTitleAndCursorOnlyFrames(t *testing.T) {
	_, fp, rec := startFake(t, 20, 3)
	fp.emit("\x1b]0;my title\x07")
	deadline := time.Now().Add(5 * time.Second)
	for {
		fs := rec.all()
		if fs[len(fs)-1].Title == "my title" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("title never arrived")
		}
		time.Sleep(2 * time.Millisecond)
	}
	fp.emit("\x1b[2;4H") // cursor move only
	deadline = time.Now().Add(5 * time.Second)
	for {
		fs := rec.all()
		l := fs[len(fs)-1]
		if l.CursorX == 3 && l.CursorY == 1 {
			if len(l.Changed) != 0 {
				t.Errorf("cursor-only frame carries rows: %+v", l.Changed)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cursor frame never arrived")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestFramesAreCoalesced(t *testing.T) {
	_, fp, rec := startFake(t, 40, 10)
	start := time.Now()
	for i := 0; i < 2000; i++ {
		fp.emit(fmt.Sprintf("line %d\r\n", i))
	}
	elapsed := time.Since(start)
	rec.waitScreen(t, func(r []string) bool { return len(r) == 10 && r[8] == "line 1999" })

	n := len(rec.all())
	max := int(elapsed/frameInterval) + 4
	if n > max {
		t.Errorf("%d frames for %v of output, want at most %d (one per %v)", n, elapsed, max, frameInterval)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for i := 1; i < len(rec.times); i++ {
		if d := rec.times[i].Sub(rec.times[i-1]); d < frameInterval-2*time.Millisecond {
			t.Errorf("frames %d and %d are only %v apart", i-1, i, d)
		}
	}
}

func TestIdleSendsNothing(t *testing.T) {
	_, fp, rec := startFake(t, 20, 3)
	fp.emit("x")
	rec.waitScreen(t, func(r []string) bool { return len(r) == 3 && r[0] == "x" })
	time.Sleep(3 * frameInterval)
	n := len(rec.all())
	time.Sleep(10 * frameInterval)
	if got := len(rec.all()); got != n {
		t.Errorf("%d frames sent while idle", got-n)
	}
}

func TestSlowSinkDoesNotBlockOutput(t *testing.T) {
	fp := newFakeProc()
	release := make(chan struct{})
	var mu sync.Mutex
	var got []protocol.PaneFrame
	h, err := Start(context.Background(), fakeSpawner{fp}, charmvt.Factory,
		pty.Spec{Argv: []string{"fake"}, Cols: 30, Rows: 5}, "p", func(f protocol.PaneFrame) {
			<-release // the first frame stalls the sink
			mu.Lock()
			got = append(got, f)
			mu.Unlock()
		})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			fp.emit(fmt.Sprintf("row %d\r\n", i))
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("program output blocked behind a slow sink")
	}
	if err := h.Input([]byte("k")); err != nil {
		t.Fatal(err)
	}
	if fp.gotInput() != "k" {
		t.Error("input blocked behind a slow sink")
	}
	close(release)

	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		rows := make([]string, 5)
		for _, f := range got {
			for _, r := range f.Changed {
				rows[r.Y] = strings.TrimSpace(ansi.Strip(r.Text))
			}
		}
		n := len(got)
		mu.Unlock()
		if rows[3] == "row 499" {
			if n > 20 {
				t.Errorf("%d frames delivered after the stall, want few (coalesced)", n)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("final screen never arrived: %q", rows)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestInputGoesStraightToPTY(t *testing.T) {
	h, fp, _ := startFake(t, 20, 3)
	for _, s := range []string{"ls", "\r", "\x1b[A"} {
		if err := h.Input([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if got := fp.gotInput(); got != "ls\r\x1b[A" {
		t.Errorf("input = %q", got)
	}
	if err := h.Input(nil); err != nil {
		t.Errorf("empty input: %v", err)
	}
}

func BenchmarkInput(b *testing.B) {
	fp := newFakeProc()
	h, err := Start(context.Background(), fakeSpawner{fp}, charmvt.Factory,
		pty.Spec{Argv: []string{"fake"}, Cols: 20, Rows: 3}, "p", nil)
	if err != nil {
		b.Fatal(err)
	}
	defer h.Close()
	key := []byte("a")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Input(key)
		if i%4096 == 0 { // keep the fake's buffer small
			fp.mu.Lock()
			fp.input = fp.input[:0]
			fp.mu.Unlock()
		}
	}
}

func TestEmulatorRepliesReachPTY(t *testing.T) {
	_, fp, _ := startFake(t, 20, 3)
	fp.emit("\x1b[c")
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(fp.gotInput(), "\x1b[?62;") {
		if time.Now().After(deadline) {
			t.Fatalf("no DA1 reply; PTY input = %q", fp.gotInput())
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestResizeSendsFullFrame(t *testing.T) {
	h, fp, rec := startFake(t, 20, 4)
	fp.emit("hi")
	rec.waitScreen(t, func(r []string) bool { return len(r) == 4 && r[0] == "hi" })
	n := len(rec.all())

	if err := h.Resize(30, 6); err != nil {
		t.Fatal(err)
	}
	fp.mu.Lock()
	rs := append([][2]int(nil), fp.resizes...)
	fp.mu.Unlock()
	if len(rs) != 1 || rs[0] != [2]int{30, 6} {
		t.Errorf("PTY resizes = %v", rs)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(rec.all()) == n {
		if time.Now().After(deadline) {
			t.Fatal("no frame after resize")
		}
		time.Sleep(2 * time.Millisecond)
	}
	f := rec.all()[n]
	if !f.Full || f.Cols != 30 || f.Rows != 6 || len(f.Changed) != 6 {
		t.Errorf("frame after resize = full:%v %dx%d rows:%d", f.Full, f.Cols, f.Rows, len(f.Changed))
	}
	if h.Resize(0, 5) == nil {
		t.Error("Resize(0,5) should fail")
	}
}

func TestSnapshot(t *testing.T) {
	h, fp, rec := startFake(t, 12, 3)
	fp.emit("snap\r\nshot")
	rec.waitScreen(t, func(r []string) bool { return len(r) == 3 && r[1] == "shot" })
	s := h.Snapshot()
	if !s.Full || s.Cols != 12 || s.Rows != 3 || len(s.Changed) != 3 {
		t.Fatalf("snapshot = %+v", s)
	}
	if got := strings.TrimSpace(ansi.Strip(s.Changed[0].Text)); got != "snap" {
		t.Errorf("row 0 = %q", got)
	}
	if s.CursorX != 4 || s.CursorY != 1 {
		t.Errorf("cursor = %d,%d", s.CursorX, s.CursorY)
	}
}

func TestExitFlow(t *testing.T) {
	h, fp, rec := startFake(t, 20, 3)
	codes := make(chan int, 2)
	h.OnExit(func(c int) { codes <- c })
	fp.emit("bye\r\n")
	fp.exit(5)

	code, err := h.Wait()
	if err != nil || code != 5 {
		t.Fatalf("Wait = %d, %v", code, err)
	}
	select {
	case c := <-codes:
		if c != 5 {
			t.Errorf("OnExit code = %d", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnExit not called")
	}
	if !h.Exited() {
		t.Error("Exited() = false")
	}
	// Output written before the exit must be in the final frame.
	if r := rec.screen(); len(r) < 1 || r[0] != "bye" {
		t.Errorf("final screen = %q", r)
	}
	// OnExit after the fact fires immediately.
	h.OnExit(func(c int) { codes <- c })
	select {
	case c := <-codes:
		if c != 5 {
			t.Errorf("late OnExit code = %d", c)
		}
	default:
		t.Error("late OnExit not called synchronously")
	}
	if got := h.Snapshot().Changed[0].Text; !strings.Contains(got, "bye") {
		t.Errorf("snapshot after exit = %q", got)
	}
}

func TestCloseKillsAndIsIdempotent(t *testing.T) {
	h, fp, _ := startFake(t, 20, 3)
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		_ = h.Close()
		_ = h.Close()
	}()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("Close deadlocked")
	}
	fp.mu.Lock()
	killed := fp.killed
	fp.mu.Unlock()
	if !killed {
		t.Error("Close did not kill the process")
	}
	if !h.Exited() {
		t.Error("host not exited after Close")
	}
}

func TestCloseFromOnExit(t *testing.T) {
	h, fp, _ := startFake(t, 20, 3)
	done := make(chan struct{})
	h.OnExit(func(int) { _ = h.Close(); close(done) })
	fp.exit(0)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Close from OnExit deadlocked")
	}
}

func TestContextCancelClosesHost(t *testing.T) {
	fp := newFakeProc()
	ctx, cancel := context.WithCancel(context.Background())
	h, err := Start(ctx, fakeSpawner{fp}, charmvt.Factory, pty.Spec{Argv: []string{"fake"}, Cols: 20, Rows: 3}, "p", nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	done := make(chan struct{})
	go func() { _, _ = h.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("cancelling the context did not stop the host")
	}
}

func TestStartErrors(t *testing.T) {
	fp := newFakeProc()
	if _, err := Start(context.Background(), fakeSpawner{fp}, charmvt.Factory, pty.Spec{Argv: []string{"x"}}, "p", nil); err == nil {
		t.Error("zero size should be rejected")
	}
}

func TestLastLines(t *testing.T) {
	h, fp, rec := startFake(t, 30, 8)
	fp.emit("\x1b[1mfirst\x1b[0m\r\n\r\nsecond line  \r\n   \r\nthird")
	rec.waitScreen(t, func(r []string) bool { return len(r) == 8 && r[4] == "third" })

	tests := []struct {
		n    int
		want []string
	}{
		{0, nil},
		{1, []string{"third"}},
		{2, []string{"second line", "third"}},
		{10, []string{"first", "second line", "third"}},
	}
	for _, tt := range tests {
		got := h.LastLines(tt.n)
		if strings.Join(got, "|") != strings.Join(tt.want, "|") {
			t.Errorf("LastLines(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
