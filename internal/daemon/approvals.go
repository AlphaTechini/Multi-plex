package daemon

import (
	"context"
	"errors"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/approval/broker"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/store"
)

// waitingForApproval is the StateDetail of a session blocked on the gate.
const waitingForApproval = "waiting for approval"

// wireBroker connects the broker to the event bus and the audit log. The
// typed code travels only in the approval.pending event.
func (d *Daemon) wireBroker() {
	if d.broker == nil {
		return
	}
	d.broker.OnPending = func(req core.ApprovalRequest, method, code string) {
		d.publish(protocol.TopicApprovalPending, protocol.ApprovalPending{Request: req, Method: method, Code: code})
	}
	d.broker.OnResolved = func(req core.ApprovalRequest, dec core.ApprovalDecision) {
		if err := d.store.LogApproval(d.ctx, req, dec); err != nil {
			d.log.Error("audit approval", "request", req.ID, "err", err)
		}
		d.publish(protocol.TopicApprovalDone, dec)
	}
}

// requestApproval asks for a human-present approval of req on behalf of s and
// blocks until it is decided. While it waits the session is blocked. The
// request's ID, session and time are filled in here.
func (d *Daemon) requestApproval(ctx context.Context, s *session, req core.ApprovalRequest) (core.ApprovalDecision, error) {
	req.ID = core.NewID("apr")
	req.SessionID = s.id
	req.At = d.now()
	d.rate.Add(s.id, req.At)

	s.mu.Lock()
	s.blockCount++
	s.mu.Unlock()
	d.setState(s, core.StateBlocked, waitingForApproval)
	defer func() {
		s.mu.Lock()
		s.blockCount--
		last := s.blockCount == 0
		s.mu.Unlock()
		if last {
			d.update(s, func(c *core.Session) {
				if c.State == core.StateBlocked && c.StateDetail == waitingForApproval {
					c.State, c.StateDetail = core.StateWorking, ""
				}
			})
		}
	}()
	if d.broker == nil {
		return core.ApprovalDecision{RequestID: req.ID, Outcome: core.Denied, Message: "no approval broker", Method: broker.MethodNone}, broker.ErrNoMethod
	}
	return d.broker.Request(ctx, req)
}

// hApprovalList lists the pending approvals.
func (d *Daemon) hApprovalList(_ context.Context, _ *conn, _ protocol.Empty) (any, error) {
	if d.broker == nil {
		return []core.ApprovalRequest{}, nil
	}
	return d.broker.Pending(), nil
}

// hApprovalRespond answers a pending approval. Approving blocks while the
// verification prompt (Windows Hello, polkit) is shown, so it runs on this
// request's own goroutine and holds no daemon lock.
func (d *Daemon) hApprovalRespond(_ context.Context, _ *conn, p protocol.ApprovalRespondParams) (any, error) {
	if d.broker == nil {
		return nil, notFound("no approval broker")
	}
	dec, err := d.broker.Respond(p.RequestID, p.Outcome, p.Message, p.Code)
	switch {
	case err == nil:
		return dec, nil
	case errors.Is(err, broker.ErrUnknownRequest):
		return nil, notFound("no pending approval %q", p.RequestID)
	case errors.Is(err, broker.ErrBusy):
		return nil, badRequest("%v", err)
	case errors.Is(err, approval.ErrCancelled), errors.Is(err, approval.ErrFailed),
		errors.Is(err, approval.ErrTimeout), errors.Is(err, approval.ErrUnavailable):
		return nil, denied("%v", err)
	}
	return nil, err
}

// hApprovalAudit returns a session's approval history.
func (d *Daemon) hApprovalAudit(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	list, err := d.store.ListApprovals(ctx, p.SessionID, 200)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []store.AuditEntry{}
	}
	return list, nil
}
