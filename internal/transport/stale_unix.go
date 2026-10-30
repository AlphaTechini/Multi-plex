//go:build !windows

package transport

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"syscall"
	"time"
)

// probeTimeout bounds the check for a live daemon on an existing socket.
const probeTimeout = 300 * time.Millisecond

// removeStaleSocket clears addr when nothing is listening on it. It returns
// ErrAddrInUse when a daemon answers. Anything at addr that is not a socket,
// or a probe that fails for another reason, is an error and is left alone.
func removeStaleSocket(addr string) error {
	fi, err := os.Lstat(addr)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("transport: stat %s: %w", addr, err)
	}
	if fi.Mode()&fs.ModeSocket == 0 {
		return fmt.Errorf("transport: %s exists and is not a socket", addr)
	}

	conn, err := (&net.Dialer{Timeout: probeTimeout}).Dial("unix", addr)
	switch {
	case err == nil:
		conn.Close()
		return fmt.Errorf("%w: %s", ErrAddrInUse, addr)
	case errors.Is(err, syscall.ECONNREFUSED):
		// Socket file with no listener behind it: stale.
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("transport: probe %s: %w", addr, err)
	}
	if err := os.Remove(addr); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("transport: remove stale socket: %w", err)
	}
	return nil
}
