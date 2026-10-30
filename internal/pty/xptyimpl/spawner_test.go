package xptyimpl

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alphatechini/multi-plex/internal/pty"
)

// TestHelperProcess is not a test: it is the child program the other tests
// spawn (the test binary running itself).
func TestHelperProcess(t *testing.T) {
	switch os.Getenv("MPX_TEST_HELPER") {
	case "":
		return
	case "echo":
		os.Stdout.WriteString("ready\r\n")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		os.Stdout.WriteString("echo:" + strings.TrimSpace(line) + "\r\n")
		os.Exit(3)
	case "env":
		os.Stdout.WriteString("TERM=" + os.Getenv("TERM") + " X=" + os.Getenv("MPX_TEST_X") + "\r\n")
		os.Exit(0)
	case "sleep":
		os.Stdout.WriteString("sleeping\r\n")
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

func helperSpec(mode string, extra ...string) pty.Spec {
	env := append(os.Environ(), "MPX_TEST_HELPER="+mode)
	env = append(env, extra...)
	return pty.Spec{
		Argv: []string{os.Args[0], "-test.run=^TestHelperProcess$"},
		Env:  env,
		Cols: 80,
		Rows: 24,
	}
}

// collector drains a process in the background.
type collector struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	done chan struct{}
}

func collect(p pty.Process) *collector {
	c := &collector{done: make(chan struct{})}
	go func() {
		defer close(c.done)
		b := make([]byte, 4096)
		for {
			n, err := p.Read(b)
			c.mu.Lock()
			c.buf.Write(b[:n])
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return c
}

func (c *collector) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

func (c *collector) waitFor(t *testing.T, sub string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(c.String(), sub) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %q; output so far: %q", sub, c.String())
}

func TestSpawnEchoAndExitCode(t *testing.T) {
	p, err := New().Spawn(context.Background(), helperSpec("echo"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Pid() <= 0 {
		t.Errorf("Pid = %d", p.Pid())
	}
	out := collect(p)
	out.waitFor(t, "ready")

	if err := p.Resize(100, 30); err != nil {
		t.Errorf("Resize: %v", err)
	}
	if _, err := io.WriteString(p, "hello\r"); err != nil {
		t.Fatal(err)
	}
	out.waitFor(t, "echo:hello")

	code, err := p.Wait()
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
	// Wait is repeatable.
	if code2, _ := p.Wait(); code2 != 3 {
		t.Errorf("second Wait = %d", code2)
	}
	// On Windows the reader only ends after Close.
	_ = p.Close()
	select {
	case <-out.done:
	case <-time.After(5 * time.Second):
		t.Fatal("reader did not end after exit and Close")
	}
}

func TestSpawnEnvAndTerm(t *testing.T) {
	spec := helperSpec("env", "TERM=linux", "MPX_TEST_X=42") // an inherited TERM must not win
	p, err := New().Spawn(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	out := collect(p)
	out.waitFor(t, "TERM=xterm-256color X=42")
	if code, _ := p.Wait(); code != 0 {
		t.Errorf("exit code = %d", code)
	}
}

func TestKillThenCloseDoesNotDeadlock(t *testing.T) {
	p, err := New().Spawn(context.Background(), helperSpec("sleep"))
	if err != nil {
		t.Fatal(err)
	}
	out := collect(p)
	out.waitFor(t, "sleeping")

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		if err := p.Kill(); err != nil {
			t.Errorf("Kill: %v", err)
		}
		if _, err := p.Wait(); err != nil {
			t.Errorf("Wait after Kill: %v", err)
		}
		_ = p.Close()
		_ = p.Close() // idempotent
		<-out.done    // reader goroutine must be released
	}()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("Kill+Close deadlocked")
	}
	if err := p.Kill(); err != nil {
		t.Errorf("Kill after exit: %v", err)
	}
}

func TestCloseUnblocksRead(t *testing.T) {
	p, err := New().Spawn(context.Background(), helperSpec("sleep"))
	if err != nil {
		t.Fatal(err)
	}
	out := collect(p)
	out.waitFor(t, "sleeping")
	_ = p.Close()
	select {
	case <-out.done:
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not unblock Read")
	}
	_ = p.Kill()
	_, _ = p.Wait()
}

func TestContextCancelKills(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p, err := New().Spawn(ctx, helperSpec("sleep"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	out := collect(p)
	out.waitFor(t, "sleeping")
	cancel()
	done := make(chan struct{})
	go func() { _, _ = p.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not stop the process")
	}
}

func TestSpawnErrors(t *testing.T) {
	sp := New()
	if _, err := sp.Spawn(context.Background(), pty.Spec{}); err == nil {
		t.Error("empty argv: want error")
	}
	if _, err := sp.Spawn(context.Background(), pty.Spec{Argv: []string{"mpx-no-such-program-xyz"}}); err == nil {
		t.Error("missing program: want error")
	}
}

func TestTerminalEnv(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"adds both", []string{"A=1"}, []string{"A=1", "TERM=xterm-256color", "COLORTERM=truecolor"}},
		{"replaces inherited", []string{"TERM=linux", "A=1", "COLORTERM=24bit"}, []string{"A=1", "TERM=xterm-256color", "COLORTERM=truecolor"}},
		{"replaces dumb", []string{"TERM=dumb"}, []string{"TERM=xterm-256color", "COLORTERM=truecolor"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := terminalEnv(tt.in)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
