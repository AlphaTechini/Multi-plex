package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/alphatechini/multi-plex/internal/ui/backend/fake"
)

// pipeInput is an input reader that serves bytes on demand and then blocks,
// like a terminal.
type pipeInput struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func newPipeInput() *pipeInput {
	r, w := io.Pipe()
	return &pipeInput{r: r, w: w}
}

func (p *pipeInput) Read(b []byte) (int, error) { return p.r.Read(b) }

// syncBuffer is a bytes.Buffer safe for the program's writer goroutine and
// the test.
type syncBuffer struct {
	mu chan struct{}
	b  bytes.Buffer
}

func newSyncBuffer() *syncBuffer {
	s := &syncBuffer{mu: make(chan struct{}, 1)}
	return s
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu <- struct{}{}
	defer func() { <-s.mu }()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu <- struct{}{}
	defer func() { <-s.mu }()
	return s.b.String()
}

// Run drives a real Bubble Tea program against the fake backend: it renders,
// takes keys through the prefix, and quits cleanly with the daemon (the fake)
// untouched.
func TestRunProgram(t *testing.T) {
	be := fake.New()
	defer be.Close()
	in := newPipeInput()
	out := newSyncBuffer()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, Options{Backend: be, WorkspaceDir: "/work"}, demoProbe,
			tea.WithInput(in), tea.WithOutput(out), tea.WithWindowSize(120, 40),
			tea.WithColorProfile(colorprofile.TrueColor),
			tea.WithEnvironment([]string{"TERM=xterm-256color", "COLORTERM=truecolor"}))
	}()
	waitFor := func(substr string) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if strings.Contains(out.String(), substr) {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("output never contained %q; tail: %q", substr, tail(out.String()))
	}
	waitFor("auth-refactor")
	waitFor("multiplex · multi-plex") // window title
	// Prefix + ? opens the help; esc closes it; prefix + q quits.
	_, _ = in.w.Write([]byte("\x1c?"))
	waitFor("SESSIONS") // sanity
	waitFor("Quitting only")
	_, _ = in.w.Write([]byte("\x1b"))
	time.Sleep(100 * time.Millisecond)
	_, _ = in.w.Write([]byte("\x1cq"))
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("program did not quit")
	}
	if !strings.Contains(out.String(), "\x1b[?1049h") {
		t.Error("alt screen not requested")
	}
}

func TestRunRequiresBackend(t *testing.T) {
	if err := Run(context.Background(), Options{}); err == nil {
		t.Fatal("Run without a backend must fail")
	}
}

func TestBuildTheme(t *testing.T) {
	th, err := buildTheme(Options{ASCII: true, ReduceMotion: true}, colorprofile.ANSI256)
	if err != nil {
		t.Fatal(err)
	}
	if th.Glyphs.Unicode || !th.Motion.Reduce || th.Profile != colorprofile.ANSI256 {
		t.Fatalf("theme = %+v", th)
	}
	if _, err := buildTheme(Options{Theme: "nope"}, colorprofile.TrueColor); err == nil {
		t.Fatal("unknown theme must fail")
	}
}

func tail(s string) string {
	if len(s) > 1500 {
		return s[len(s)-1500:]
	}
	return s
}

// With BenchFirstFrame the program prints the time to the first frame and
// quits by itself, without waiting for data.
func TestRunBenchFirstFrame(t *testing.T) {
	be := fake.New()
	defer be.Close()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	ProcessStart = time.Now()
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = run(ctx, Options{Backend: be, BenchFirstFrame: true}, demoProbe,
		tea.WithInput(newPipeInput()), tea.WithOutput(newSyncBuffer()), tea.WithWindowSize(120, 40))
	w.Close()
	os.Stderr = old
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	data, _ := io.ReadAll(r)
	line := strings.TrimSpace(string(data))
	var ms float64
	if n, err := fmt.Sscanf(line, "first-frame-ms %f", &ms); n != 1 || err != nil {
		t.Fatalf("stderr = %q", line)
	}
	if ms <= 0 || ms > 5000 {
		t.Fatalf("first-frame-ms = %v", ms)
	}
}
