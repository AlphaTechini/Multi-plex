package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"runtime/debug"
	"sync"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// conn is one client connection.
type conn struct {
	d   *Daemon
	nc  net.Conn
	enc protocol.Encoder

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	sub    *subscriber
	client string

	wg sync.WaitGroup // request goroutines
}

// serveConn runs one connection until it ends. Each request runs in its own
// goroutine, because hook calls block on approvals for minutes; requests that
// must keep their order (pane input, resize) and are quick run inline.
func (d *Daemon) serveConn(nc net.Conn) {
	ctx, cancel := context.WithCancel(d.ctx)
	c := &conn{d: d, nc: nc, enc: protocol.JSON.NewEncoder(nc), ctx: ctx, cancel: cancel}
	d.mu.Lock()
	d.conns[c] = struct{}{}
	d.mu.Unlock()

	defer func() {
		cancel()
		c.mu.Lock()
		sub := c.sub
		c.mu.Unlock()
		if sub != nil {
			d.bus.unsubscribe(sub)
		}
		_ = nc.Close()
		c.wg.Wait()
		d.mu.Lock()
		delete(d.conns, c)
		d.mu.Unlock()
	}()

	dec := protocol.JSON.NewDecoder(nc)
	for {
		var f protocol.Frame
		if err := dec.Decode(&f); err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				d.log.Debug("connection closed", "err", err)
			}
			return
		}
		if f.Type != protocol.TypeRequest {
			continue
		}
		rt, ok := d.routes[f.Method]
		if !ok {
			c.respond(f.ID, nil, badRequest("unknown method %q", f.Method))
			continue
		}
		if rt.inline {
			c.run(rt, f)
			continue
		}
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.run(rt, f)
		}()
	}
}

// run executes one request and writes its response.
func (c *conn) run(rt route, f protocol.Frame) {
	defer func() {
		if r := recover(); r != nil {
			c.d.log.Error("handler panicked", "method", f.Method, "panic", r, "stack", string(debug.Stack()))
			c.respond(f.ID, nil, &protocol.Error{Code: protocol.CodeInternal, Message: "internal error"})
		}
	}()
	res, err := rt.h(c.ctx, c, f.Data)
	c.respond(f.ID, res, err)
}

// respond writes the response for request id.
func (c *conn) respond(id uint64, result any, err error) {
	fr := protocol.Frame{Type: protocol.TypeResponse, ID: id}
	if err != nil {
		fr.Error = toProtocolError(err)
	} else if result != nil {
		data, merr := json.Marshal(result)
		if merr != nil {
			fr.Error = &protocol.Error{Code: protocol.CodeInternal, Message: "encode result: " + merr.Error()}
		} else {
			fr.Data = data
		}
	}
	if werr := c.enc.Encode(&fr); werr != nil {
		c.cancel()
		_ = c.nc.Close()
	}
}

// writeEvents drains the subscriber's queue onto the connection until the
// connection ends.
func (c *conn) writeEvents(s *subscriber) {
	for {
		select {
		case it := <-s.q:
			fr := protocol.Frame{Type: protocol.TypeEvent, Topic: it.topic, Data: it.data}
			if err := c.enc.Encode(&fr); err != nil {
				c.cancel()
				_ = c.nc.Close()
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}
