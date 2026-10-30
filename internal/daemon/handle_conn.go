package daemon

import (
	"context"
	"os"
	"time"

	"github.com/alphatechini/multi-plex/internal/protocol"
)

// hHello answers the opening handshake.
func (d *Daemon) hHello(_ context.Context, c *conn, p protocol.HelloParams) (any, error) {
	c.mu.Lock()
	c.client = p.Client
	c.mu.Unlock()
	return protocol.HelloResult{
		Protocol: protocol.Version,
		Version:  d.version,
		PID:      os.Getpid(),
		Approver: d.approver,
	}, nil
}

// hSubscribe registers the connection on the event bus. Approvals that are
// already pending are replayed to the new subscriber, so a TUI that attaches
// late still shows them (and the typed code, which only this event carries).
func (d *Daemon) hSubscribe(_ context.Context, c *conn, p protocol.SubscribeParams) (any, error) {
	c.mu.Lock()
	if c.sub != nil {
		c.mu.Unlock()
		d.bus.unsubscribe(c.sub)
		c.mu.Lock()
	}
	sub := d.bus.subscribe(p.Topics)
	c.sub = sub
	c.mu.Unlock()
	go c.writeEvents(sub)

	// The subscriber has seen none of the frames so far: make the next frame
	// of every pane a full one, so it never builds a screen from a delta.
	d.mu.RLock()
	for id := range d.panes {
		sub.markNeedFull(id)
	}
	d.mu.RUnlock()

	if d.broker != nil && sub.wants(protocol.TopicApprovalPending) {
		for _, pd := range d.broker.PendingDetails() {
			d.bus.replay(sub, protocol.TopicApprovalPending, protocol.ApprovalPending{
				Request: pd.Request, Method: pd.Method, Code: pd.Code,
			})
		}
	}
	return protocol.Empty{}, nil
}

// hShutdown stops the daemon after answering.
func (d *Daemon) hShutdown(_ context.Context, _ *conn, _ protocol.Empty) (any, error) {
	d.log.Info("shutdown requested")
	time.AfterFunc(100*time.Millisecond, d.Shutdown)
	return protocol.Empty{}, nil
}
