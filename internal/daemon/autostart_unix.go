//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

// detach puts the child in its own session so it survives the terminal that
// started it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
