package term

import (
	"context"
	"io"
	"sync"

	"github.com/alphatechini/multi-plex/internal/pty"
)

// fakeProc is an in-memory pty.Process: output written with emit is what the
// host reads, and input written by the host is collected in input.
type fakeProc struct {
	outR *io.PipeReader
	outW *io.PipeWriter

	mu      sync.Mutex
	input   []byte
	resizes [][2]int
	killed  bool

	exitOnce sync.Once
	exitCh   chan struct{}
	code     int

	closeOnce sync.Once
}

func newFakeProc() *fakeProc {
	r, w := io.Pipe()
	return &fakeProc{outR: r, outW: w, exitCh: make(chan struct{})}
}

// emit sends program output to the host.
func (f *fakeProc) emit(s string) { _, _ = f.outW.Write([]byte(s)) }

// exit ends the process with code. Like a Unix PTY, the output side then
// reports EOF once everything written has been read.
func (f *fakeProc) exit(code int) {
	f.exitOnce.Do(func() {
		f.code = code
		close(f.exitCh)
		_ = f.outW.Close()
	})
}

func (f *fakeProc) Read(p []byte) (int, error) { return f.outR.Read(p) }

func (f *fakeProc) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.input = append(f.input, p...)
	return len(p), nil
}

func (f *fakeProc) Resize(cols, rows int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resizes = append(f.resizes, [2]int{cols, rows})
	return nil
}

func (f *fakeProc) Wait() (int, error) { <-f.exitCh; return f.code, nil }

func (f *fakeProc) Kill() error {
	f.mu.Lock()
	f.killed = true
	f.mu.Unlock()
	f.exit(137)
	return nil
}

func (f *fakeProc) Close() error {
	f.closeOnce.Do(func() {
		f.exit(137) // closing hangs up
		_ = f.outR.Close()
	})
	return nil
}

func (f *fakeProc) Pid() int { return 4242 }

func (f *fakeProc) gotInput() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return string(f.input)
}

// fakeSpawner returns a prepared fakeProc.
type fakeSpawner struct{ p *fakeProc }

func (s fakeSpawner) Spawn(context.Context, pty.Spec) (pty.Process, error) { return s.p, nil }
