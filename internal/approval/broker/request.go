package broker

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
)

// Request registers req, announces it through OnPending and blocks until a UI
// client decides it through Respond, the broker's Timeout passes (denied with
// "approval timed out") or ctx ends (denied, and ctx's error is returned).
// A returned decision is final and already audited. An empty or duplicate ID
// returns an error and no decision; with no usable method the request is
// denied immediately and ErrNoMethod is returned with that decision.
func (b *Broker) Request(ctx context.Context, req core.ApprovalRequest) (core.ApprovalDecision, error) {
	if req.ID == "" {
		return core.ApprovalDecision{}, fmt.Errorf("approval: request needs an id")
	}
	v, method := b.choose(ctx, nil)

	rctx, cancel := context.WithTimeout(ctx, b.timeout())
	defer cancel() // also aborts a verification still showing its prompt
	p := &pendingReq{
		req: req, ctx: rctx, verifier: v, method: method,
		tried:  map[string]bool{},
		result: make(chan core.ApprovalDecision, 1),
	}

	b.mu.Lock()
	if _, dup := b.reqs[req.ID]; dup {
		b.mu.Unlock()
		return core.ApprovalDecision{}, ErrDuplicate
	}
	if method != "" {
		if v == nil {
			p.code = b.typed.Issue(req.ID)
		}
		b.reqs[req.ID] = p
	}
	b.mu.Unlock()

	if method == "" {
		d := core.ApprovalDecision{RequestID: req.ID, Outcome: core.Denied, Message: "no approval method available", Method: MethodNone, At: b.now()}
		if b.audit != nil {
			b.audit(req, d)
		}
		if b.OnResolved != nil {
			b.OnResolved(req, d)
		}
		return d, ErrNoMethod
	}

	if b.OnPending != nil {
		b.OnPending(req, p.method, p.code)
	}

	select {
	case d := <-p.result:
		return d, nil
	case <-rctx.Done():
		if ctx.Err() != nil {
			return b.finalize(p, core.Denied, "approval request cancelled", MethodNone), ctx.Err()
		}
		return b.finalize(p, core.Denied, "approval timed out", MethodNone), nil
	}
}
