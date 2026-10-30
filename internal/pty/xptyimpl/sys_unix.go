//go:build !windows

package xptyimpl

import (
	"errors"
	"io"
	"os"
	"syscall"

	"github.com/charmbracelet/x/xpty"
)

// sysProcAttr puts the child in its own session with the PTY as controlling
// terminal, so shells get job control and Ctrl+C reaches the foreground job.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true, Setctty: true}
}

// closeSlave closes our copy of the slave end after Start.
func closeSlave(p xpty.Pty) {
	if up, ok := p.(*xpty.UnixPty); ok {
		_ = up.Slave().Close()
	}
}

// isEOF reports whether a master read error means the terminal is finished.
func isEOF(err error) bool {
	return errors.Is(err, syscall.EIO) || errors.Is(err, os.ErrClosed) || errors.Is(err, io.EOF)
}

// killProcess kills the process group (the child is a session leader, so its
// pgid is its pid), falling back to the process itself.
func killProcess(pr *os.Process) error {
	if pr == nil {
		return nil
	}
	if err := syscall.Kill(-pr.Pid, syscall.SIGKILL); err == nil {
		return nil
	}
	return pr.Kill()
}

// exitCode returns the exit code, or 128+signal when the process was killed
// by a signal.
func exitCode(ps *os.ProcessState) int {
	if ps == nil {
		return -1
	}
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ps.ExitCode()
}
