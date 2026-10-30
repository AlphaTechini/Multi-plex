package client

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Shutdown asks the daemon to stop. The daemon answers, then exits.
func (c *Client) Shutdown(ctx context.Context) error {
	return c.Call(ctx, protocol.MethodShutdown, protocol.Empty{}, nil)
}
