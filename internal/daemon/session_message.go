package daemon

import (
	"context"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// interruptGap is how long to wait between interrupting an agent and typing
// into it, so the agent is back at its input.
const interruptGap = 200 * time.Millisecond

// agentPane returns the session's running agent pane.
func (d *Daemon) agentPane(s *session) (*pane, error) {
	id := s.snap().AgentPane
	if id == "" {
		return nil, badRequest("the agent of session %s is not running", s.snap().Slug)
	}
	p, err := d.paneByID(id)
	if err != nil {
		return nil, badRequest("the agent of session %s is not running", s.snap().Slug)
	}
	return p, nil
}

// writeAgent types raw bytes into the session's agent.
func (d *Daemon) writeAgent(s *session, b []byte) error {
	p, err := d.agentPane(s)
	if err != nil {
		return err
	}
	if err := p.write(b); err != nil {
		return badRequest("%v", err)
	}
	return nil
}

// steer types a message into the agent, interrupting it first when asked.
func (d *Daemon) steer(ctx context.Context, s *session, text string, interrupt bool) error {
	a, err := adapterFor(s.snap())
	if err != nil {
		return err
	}
	if interrupt {
		if err := d.writeAgent(s, a.Interrupt()); err != nil {
			return err
		}
		select {
		case <-time.After(interruptGap):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return d.writeAgent(s, a.TypeMessage(text))
}

// hSessionMessage steers a session's agent.
func (d *Daemon) hSessionMessage(ctx context.Context, _ *conn, p protocol.SessionMessageParams) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	if p.Text == "" {
		return nil, badRequest("text is required")
	}
	if err := d.steer(ctx, s, p.Text, p.Interrupt); err != nil {
		return nil, err
	}
	return protocol.Empty{}, nil
}

// hSessionInterrupt interrupts the agent's current step.
func (d *Daemon) hSessionInterrupt(_ context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	a, err := adapterFor(s.snap())
	if err != nil {
		return nil, err
	}
	if err := d.writeAgent(s, a.Interrupt()); err != nil {
		return nil, err
	}
	return protocol.Empty{}, nil
}

// hSessionSeen clears the "done and not viewed" mark.
func (d *Daemon) hSessionSeen(_ context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	d.update(s, func(c *core.Session) { c.Unseen = false })
	return protocol.Empty{}, nil
}

// hSessionFollow turns follow mode on or off.
func (d *Daemon) hSessionFollow(_ context.Context, _ *conn, p protocol.SessionFollowParams) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	d.update(s, func(c *core.Session) { c.Follow = p.On })
	return protocol.Empty{}, nil
}
