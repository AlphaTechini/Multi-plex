// Package clientbackend implements backend.Backend over the daemon client
// (internal/client): one typed call per protocol method, and the daemon's
// event stream converted to backend events.
package clientbackend

import (
	"context"
	"sync"

	"github.com/alphatechini/multi-plex/internal/client"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
)

// Backend talks to a running daemon through a client.Client.
type Backend struct {
	c      *client.Client
	events chan backend.Event

	subOnce sync.Once
	subErr  error
}

var _ backend.Backend = (*Backend)(nil)

// New wraps an established daemon client and starts converting its events.
// The subscription to the daemon's topics is made on the first
// OpenWorkspace call (or an explicit Subscribe).
func New(c *client.Client) *Backend {
	b := &Backend{c: c, events: make(chan backend.Event, eventBuffer)}
	go b.pump()
	return b
}

// Events returns the converted event stream. It is closed when the daemon
// connection ends.
func (b *Backend) Events() <-chan backend.Event { return b.events }

// Subscribe asks the daemon to push all event topics. It is idempotent.
func (b *Backend) Subscribe(ctx context.Context) error {
	b.subOnce.Do(func() { b.subErr = b.c.Subscribe(ctx) })
	return b.subErr
}

// Close closes the connection to the daemon (the daemon keeps running).
func (b *Backend) Close() error { return b.c.Close() }

// call is the shared request helper.
func (b *Backend) call(ctx context.Context, method string, params, out any) error {
	if params == nil {
		params = protocol.Empty{}
	}
	return b.c.Call(ctx, method, params, out)
}
