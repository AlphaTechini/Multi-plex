package client

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// MCP forwards one MCP tool call to the daemon.
func (c *Client) MCP(ctx context.Context, p protocol.MCPParams) (protocol.MCPResult, error) {
	var res protocol.MCPResult
	err := c.Call(ctx, protocol.MethodMCP, p, &res)
	return res, err
}
