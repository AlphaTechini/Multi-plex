//go:build !windows

package gitcli

import "os/exec"

// hideWindow is a no-op outside Windows.
func hideWindow(*exec.Cmd) {}
