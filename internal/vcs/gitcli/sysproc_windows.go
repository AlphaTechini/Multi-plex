//go:build windows

package gitcli

import (
	"os/exec"
	"syscall"
)

// hideWindow stops git.exe from flashing a console window when the caller has
// no console.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
