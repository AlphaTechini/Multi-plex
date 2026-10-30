package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/user"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
)

// pipeSecurity returns an SDDL DACL with protection set (D:P) that grants
// full access to the current user's SID and to SYSTEM only. An explicit SID
// is used instead of OWNER RIGHTS because an elevated process's objects are
// owned by Administrators, which would lock out the same user's normal
// processes.
func pipeSecurity() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("transport: current user: %w", err)
	}
	return "D:P(A;;GA;;;" + u.Uid + ")(A;;GA;;;SY)", nil
}

// probeTimeout bounds the check for a live daemon on an existing pipe.
const probeTimeout = 300 * time.Millisecond

// Listen creates a named pipe at addr, such as \\.\pipe\multiplex-name,
// restricted to the current user. If a daemon already serves the pipe, it
// returns ErrAddrInUse.
func Listen(addr string) (net.Listener, error) {
	if err := probePipe(addr); err != nil {
		return nil, err
	}
	sd, err := pipeSecurity()
	if err != nil {
		return nil, err
	}
	ln, err := winio.ListenPipe(addr, &winio.PipeConfig{SecurityDescriptor: sd})
	if err != nil {
		return nil, fmt.Errorf("transport: listen %s: %w", addr, err)
	}
	return ln, nil
}

// probePipe returns ErrAddrInUse when a daemon answers at addr or its
// instances are all busy. A missing pipe is not an error.
func probePipe(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	conn, err := winio.DialPipeContext(ctx, addr)
	switch {
	case err == nil:
		conn.Close()
		return fmt.Errorf("%w: %s", ErrAddrInUse, addr)
	case errors.Is(err, syscall.ERROR_FILE_NOT_FOUND):
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%w: %s (instances busy)", ErrAddrInUse, addr)
	default:
		return fmt.Errorf("transport: probe %s: %w", addr, err)
	}
}
