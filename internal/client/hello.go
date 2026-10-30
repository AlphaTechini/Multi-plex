package client

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Hello opens the connection: it tells the daemon what kind of client this is
// ("tui", "hook", "mcp", ...) and returns the daemon's version and approver.
func (c *Client) Hello(ctx context.Context, name string) (protocol.HelloResult, error) {
	var res protocol.HelloResult
	err := c.Call(ctx, protocol.MethodHello, protocol.HelloParams{Client: name, Protocol: protocol.Version}, &res)
	return res, err
}
