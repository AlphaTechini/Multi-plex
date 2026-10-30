package xptyimpl

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/charmbracelet/x/xpty"

	"github.com/alphatechini/multi-plex/internal/pty"
)

// process is a running program attached to an xpty pseudo-terminal.
type process struct {
	pty xpty.Pty
	cmd *exec.Cmd

	done    chan struct{} // closed once the process has been reaped
	code    int
	waitErr error

	closeOnce sync.Once
	closeErr  error
}

var _ pty.Process = (*process)(nil)

func newProcess(p xpty.Pty, cmd *exec.Cmd) *process {
	return &process{pty: p, cmd: cmd, done: make(chan struct{})}
}

// reap waits for the child with xpty's helper (exec.Cmd.Wait does not work
// with ConPTY) and publishes the result. It runs for every process, so a
// caller that never calls Wait still does not leak a zombie.
func (p *process) reap() {
	err := xpty.WaitProcess(context.Background(), p.cmd)
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		err = nil // a non-zero exit is a result, not a failure to wait
	default:
		p.waitErr = err
	}
	p.code = exitCode(p.cmd.ProcessState)
	close(p.done)
}

// Read returns terminal output. It reports io.EOF when the child side has
// closed (EIO on Unix, a broken pipe on Windows) or the terminal was closed.
func (p *process) Read(b []byte) (int, error) {
	n, err := p.pty.Read(b)
	if err != nil && isEOF(err) {
		err = io.EOF
	}
	return n, err
}

// Write sends input to the program.
func (p *process) Write(b []byte) (int, error) { return p.pty.Write(b) }

// Resize changes the terminal size.
func (p *process) Resize(cols, rows int) error { return p.pty.Resize(cols, rows) }

// Wait blocks until the process exits and returns its exit code. On Windows
// the output pipe does not reach EOF by itself after the exit: the caller
// must Close the process once it has drained the output it wants.
func (p *process) Wait() (int, error) {
	<-p.done
	return p.code, p.waitErr
}

// Kill stops the process (on Unix its whole process group). Killing a
// process that already exited is not an error.
func (p *process) Kill() error {
	select {
	case <-p.done:
		return nil
	default:
	}
	if err := killProcess(p.cmd.Process); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

// Close releases the terminal. Like a real hangup it kills the program if it
// is still running: on Unix the master descriptor is only really closed once
// a blocked Read returns, which needs the child side to go away. It is safe
// to call more than once and unblocks a goroutine blocked in Read.
func (p *process) Close() error {
	p.closeOnce.Do(func() {
		_ = p.Kill()
		err := p.pty.Close()
		if errors.Is(err, os.ErrClosed) {
			err = nil // the slave end was already closed after Start
		}
		p.closeErr = err
	})
	return p.closeErr
}

// Pid returns the process id.
func (p *process) Pid() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}
