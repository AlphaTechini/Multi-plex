//go:build !windows

package gitremote

import "os/exec"

func hideWindow(*exec.Cmd) {}
