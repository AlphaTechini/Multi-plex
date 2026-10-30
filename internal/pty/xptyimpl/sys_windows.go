//go:build windows

package xptyimpl

import (
	"errors"
	"io"
	"os"
	"syscall"

	"github.com/charmbracelet/x/xpty"
	"golang.org/x/sys/windows"
)

// sysProcAttr needs nothing on Windows: ConPTY attaches the console.
func sysProcAttr() *syscall.SysProcAttr { return nil }

// closeSlave is a no-op: ConPTY has no slave end to close.
func closeSlave(xpty.Pty) {}

// isEOF reports whether a read error means the pseudo console is finished.
func isEOF(err error) bool {
	return errors.Is(err, windows.ERROR_BROKEN_PIPE) ||
		errors.Is(err, windows.ERROR_PIPE_NOT_CONNECTED) ||
		errors.Is(err, os.ErrClosed) || errors.Is(err, io.EOF)
}

// killProcess kills the process. Anything it started that is attached to the
// pseudo console goes away when the console is closed.
func killProcess(pr *os.Process) error {
	if pr == nil {
		return nil
	}
	return pr.Kill()
}

// exitCode returns the process exit code.
func exitCode(ps *os.ProcessState) int {
	if ps == nil {
		return -1
	}
	return ps.ExitCode()
}
