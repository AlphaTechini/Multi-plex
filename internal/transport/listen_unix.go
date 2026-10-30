//go:build !windows

package transport

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// Listen opens a Unix domain socket at addr. The parent directory is created
// with mode 0700 and the socket is set to 0600. A stale socket left by a dead
// daemon is removed. If a live daemon answers, Listen returns ErrAddrInUse.
func Listen(addr string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(addr), 0o700); err != nil {
		return nil, fmt.Errorf("transport: create socket dir: %w", err)
	}
	if err := removeStaleSocket(addr); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("transport: listen %s: %w", addr, err)
	}
	if err := os.Chmod(addr, 0o600); err != nil {
		ln.Close()
		return nil, fmt.Errorf("transport: chmod socket: %w", err)
	}
	return ln, nil
}
