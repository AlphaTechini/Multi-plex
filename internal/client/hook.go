package client

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Hook sends one agent hook event to the daemon and returns its answer. For a
// pre-tool event the call can block for minutes while a human decides, so the
// caller chooses the context.
func (c *Client) Hook(ctx context.Context, p protocol.HookParams) (agent.HookResponse, error) {
	var res agent.HookResponse
	err := c.Call(ctx, protocol.MethodHook, p, &res)
	return res, err
}
