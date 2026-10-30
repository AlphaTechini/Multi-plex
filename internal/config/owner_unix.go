//go:build !windows

package config

import (
	"fmt"
	"os"
	"syscall"
)

// checkOwner refuses a directory owned by another user, so a runtime
// directory pre-created in a shared temp folder cannot hijack the socket.
func checkOwner(dir string) error {
	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if int(st.Uid) != os.Getuid() {
		return fmt.Errorf("config: %s is owned by uid %d, not the current user", dir, st.Uid)
	}
	return nil
}
