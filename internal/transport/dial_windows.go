package transport

import (
	"context"
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
)

// Dial connects to the named pipe at addr, retrying while its instances are
// busy, until ctx is done.
func Dial(ctx context.Context, addr string) (net.Conn, error) {
	conn, err := winio.DialPipeContext(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("transport: dial %s: %w", addr, err)
	}
	return conn, nil
}
