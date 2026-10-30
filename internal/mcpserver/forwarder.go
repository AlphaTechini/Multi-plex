package mcpserver

import (
	"context"
	"errors"
	"sync"

	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Caller sends one tool call to the daemon. *client.Client implements it
// through its MCP method.
type Caller interface {
	MCP(ctx context.Context, p protocol.MCPParams) (protocol.MCPResult, error)
}

// Dialer connects to the daemon.
type Dialer func(ctx context.Context) (*client.Client, error)

// Forwarder is a Caller that dials the daemon on first use and again after the
// connection was lost, so an MCP server that outlives a daemon restart keeps
// working.
type Forwarder struct {
	dial Dialer

	mu sync.Mutex
	c  *client.Client
}

// NewForwarder returns a Forwarder that connects with dial.
func NewForwarder(dial Dialer) *Forwarder { return &Forwarder{dial: dial} }

// MCP forwards the call, redialing once if the connection had dropped.
func (f *Forwarder) MCP(ctx context.Context, p protocol.MCPParams) (protocol.MCPResult, error) {
	for attempt := 0; ; attempt++ {
		c, err := f.conn(ctx)
		if err != nil {
			return protocol.MCPResult{}, err
		}
		res, err := c.MCP(ctx, p)
		if errors.Is(err, client.ErrClosed) && attempt == 0 {
			f.drop(c)
			continue
		}
		return res, err
	}
}

func (f *Forwarder) conn(ctx context.Context) (*client.Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.c != nil {
		select {
		case <-f.c.Done():
			f.c = nil
		default:
			return f.c, nil
		}
	}
	c, err := f.dial(ctx)
	if err != nil {
		return nil, err
	}
	f.c = c
	return c, nil
}

func (f *Forwarder) drop(c *client.Client) {
	f.mu.Lock()
	if f.c == c {
		f.c = nil
	}
	f.mu.Unlock()
	_ = c.Close()
}

// Close closes the daemon connection, if any.
func (f *Forwarder) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.c == nil {
		return nil
	}
	err := f.c.Close()
	f.c = nil
	return err
}

var _ Caller = (*Forwarder)(nil)
