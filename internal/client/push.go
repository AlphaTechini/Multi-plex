package client

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Push asks the daemon to push a session's branch. It blocks until a human
// approves or denies it, so pass a context without a short deadline.
func (c *Client) Push(ctx context.Context, id core.SessionID) (protocol.PushResult, error) {
	var res protocol.PushResult
	err := c.Call(ctx, protocol.MethodSessionPush, protocol.SessionRef{SessionID: id}, &res)
	return res, err
}
