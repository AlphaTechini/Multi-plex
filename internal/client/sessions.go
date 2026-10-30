package client

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// CreateSession creates a session and starts its agent.
func (c *Client) CreateSession(ctx context.Context, p protocol.SessionCreateParams) (core.Session, error) {
	var s core.Session
	err := c.Call(ctx, protocol.MethodSessionCreate, p, &s)
	return s, err
}

// Sessions lists sessions, of one workspace or (with "") of all.
func (c *Client) Sessions(ctx context.Context, ws core.WorkspaceID) ([]core.Session, error) {
	var out []core.Session
	err := c.Call(ctx, protocol.MethodSessionList, protocol.SessionListParams{WorkspaceID: ws}, &out)
	return out, err
}

// SessionDetail returns everything the Overview shows for one session.
func (c *Client) SessionDetail(ctx context.Context, id core.SessionID) (protocol.SessionDetail, error) {
	var d protocol.SessionDetail
	err := c.Call(ctx, protocol.MethodSessionGet, protocol.SessionRef{SessionID: id}, &d)
	return d, err
}

// RemoveSession stops a session and removes its worktree and branch.
func (c *Client) RemoveSession(ctx context.Context, id core.SessionID) error {
	return c.Call(ctx, protocol.MethodSessionRemove, protocol.SessionRef{SessionID: id}, nil)
}
