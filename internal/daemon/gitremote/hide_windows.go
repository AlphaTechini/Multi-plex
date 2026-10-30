//go:build windows

package gitremote

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps git from flashing a console window.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
