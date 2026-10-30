// Package client is a typed client for the daemon protocol. The TUI,
// `mpx hook` and `mpx mcp` all talk to the daemon through it.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/transport"
)

// ErrClosed is returned by calls on a closed client.
var ErrClosed = errors.New("client: connection closed")

// Event is one event pushed by the daemon.
type Event struct {
	Topic string
	Data  json.RawMessage
}

// Client is a connection to the daemon. It is safe for concurrent use.
type Client struct {
	conn    net.Conn
	enc     protocol.Encoder
	nextID  atomic.Uint64
	mu      sync.Mutex
	pending map[uint64]chan *protocol.Frame
	events  chan Event
	done    chan struct{}
	err     error
}

// Dial connects to the daemon at addr.
func Dial(ctx context.Context, addr string) (*Client, error) {
	conn, err := transport.Dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	return New(conn, protocol.JSON), nil
}

// New wraps an established connection.
func New(conn net.Conn, codec protocol.Codec) *Client {
	c := &Client{
		conn:    conn,
		enc:     codec.NewEncoder(conn),
		pending: map[uint64]chan *protocol.Frame{},
		events:  make(chan Event, 1024),
		done:    make(chan struct{}),
	}
	go c.readLoop(codec.NewDecoder(conn))
	return c
}

// Events returns the channel of pushed events. It is closed when the
// connection ends. Call Subscribe first.
func (c *Client) Events() <-chan Event { return c.events }

// Done is closed when the connection ends.
func (c *Client) Done() <-chan struct{} { return c.done }

// Err returns why the connection ended, after Done is closed.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close closes the connection.
func (c *Client) Close() error { return c.conn.Close() }

// Call sends a request and decodes the result into out (which may be nil).
func (c *Client) Call(ctx context.Context, method string, params, out any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("client: encode %s: %w", method, err)
	}
	id := c.nextID.Add(1)
	ch := make(chan *protocol.Frame, 1)
	c.mu.Lock()
	if c.pending == nil {
		c.mu.Unlock()
		return ErrClosed
	}
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.pending != nil {
			delete(c.pending, id)
		}
		c.mu.Unlock()
	}()

	if err := c.enc.Encode(&protocol.Frame{Type: protocol.TypeRequest, ID: id, Method: method, Data: data}); err != nil {
		return fmt.Errorf("client: send %s: %w", method, err)
	}
	select {
	case f, ok := <-ch:
		if !ok {
			return ErrClosed
		}
		return decodeResponse(f, out)
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		// A response that arrived just before the connection closed is still
		// valid: select picks among ready cases at random.
		select {
		case f, ok := <-ch:
			if ok {
				return decodeResponse(f, out)
			}
		default:
		}
		return ErrClosed
	}
}

// decodeResponse turns a response frame into an error or the decoded result.
func decodeResponse(f *protocol.Frame, out any) error {
	if f.Error != nil {
		return f.Error
	}
	if out == nil || len(f.Data) == 0 {
		return nil
	}
	return json.Unmarshal(f.Data, out)
}

// Subscribe asks the daemon to push events for topics (all when empty).
func (c *Client) Subscribe(ctx context.Context, topics ...string) error {
	return c.Call(ctx, protocol.MethodSubscribe, protocol.SubscribeParams{Topics: topics}, nil)
}

func (c *Client) readLoop(dec protocol.Decoder) {
	var err error
	for {
		var f protocol.Frame
		if err = dec.Decode(&f); err != nil {
			break
		}
		switch f.Type {
		case protocol.TypeResponse:
			c.mu.Lock()
			ch := c.pending[f.ID]
			c.mu.Unlock()
			if ch != nil {
				fr := f
				ch <- &fr
			}
		case protocol.TypeEvent:
			c.events <- Event{Topic: f.Topic, Data: f.Data}
		}
	}
	c.mu.Lock()
	c.err = err
	c.pending = nil
	c.mu.Unlock()
	close(c.done)
	close(c.events)
}
