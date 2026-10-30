//go:build !windows

package transport

import (
	"context"
	"fmt"
	"net"
)

// Dial connects to the Unix domain socket at addr, honouring ctx.
func Dial(ctx context.Context, addr string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", addr)
	if err != nil {
		return nil, fmt.Errorf("transport: dial %s: %w", addr, err)
	}
	return conn, nil
}
