package broker

import (
	"errors"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/core"
)

// Respond delivers a UI client's answer for a pending request.
//
// Deny decides immediately (message goes back to the agent) and needs no
// verification. Approve runs the request's method and BLOCKS while the prompt
// is up, so call it from its own goroutine. Only a verified approve returns an
// Approved decision. If verification is cancelled or fails, the request stays
// pending so the user can retry: Respond returns a zero decision and the
// verifier's error (errors.Is works against the approval.Err* values). When
// failures reach MaxFailures the request is denied and that final decision is
// returned with a nil error. If the chosen method turns out to be unavailable,
// the broker moves to the next one (re-announcing through OnPending) and
// returns ErrUnavailable so the client can prompt again.
func (b *Broker) Respond(requestID string, outcome core.ApprovalOutcome, message, code string) (core.ApprovalDecision, error) {
	b.mu.Lock()
	p := b.reqs[requestID]
	if p == nil {
		b.mu.Unlock()
		return core.ApprovalDecision{}, ErrUnknownRequest
	}
	switch outcome {
	case core.Denied:
		b.mu.Unlock()
		return b.finalize(p, core.Denied, message, MethodNone), nil
	case core.Approved:
	default:
		b.mu.Unlock()
		return core.ApprovalDecision{}, fmt.Errorf("approval: unknown outcome %q", outcome)
	}
	if p.verifying {
		b.mu.Unlock()
		return core.ApprovalDecision{}, ErrBusy
	}
	p.verifying = true
	v, method := p.verifier, p.method
	b.mu.Unlock()

	var err error
	if v != nil {
		err = v.Verify(p.ctx, p.req, describe(p.req))
	} else {
		err = b.typed.Submit(requestID, code)
	}

	b.mu.Lock()
	p.verifying = false
	if p.done { // denied or timed out while the prompt was up
		d := p.decision
		b.mu.Unlock()
		return d, nil
	}
	b.mu.Unlock()

	switch {
	case err == nil:
		return b.finalize(p, core.Approved, "", method), nil
	case errors.Is(err, approval.ErrCancelled):
		return core.ApprovalDecision{}, err
	case errors.Is(err, approval.ErrUnavailable):
		if d, final := b.fallBack(p, method); final {
			return d, nil
		}
		return core.ApprovalDecision{}, err
	case v == nil && errors.Is(err, approval.ErrTimeout):
		b.reissue(p) // the code expired before the request did
		return core.ApprovalDecision{}, err
	}

	b.mu.Lock()
	p.failures++
	exhausted := p.failures >= b.maxFailures()
	b.mu.Unlock()
	if exhausted {
		return b.finalize(p, core.Denied, "approval verification failed", method), nil
	}
	return core.ApprovalDecision{}, err
}

// fallBack moves p to the next usable method after method proved unavailable.
// If none is left the request is denied and that decision is returned with true.
func (b *Broker) fallBack(p *pendingReq, method string) (core.ApprovalDecision, bool) {
	b.mu.Lock()
	p.tried[method] = true
	tried := make(map[string]bool, len(p.tried))
	for k := range p.tried {
		tried[k] = true
	}
	b.mu.Unlock()

	v, m := b.choose(p.ctx, tried) // may run a helper: not under the lock
	if m == "" {
		return b.finalize(p, core.Denied, "no approval method available", MethodNone), true
	}
	b.mu.Lock()
	p.verifier, p.method, p.code = v, m, ""
	if v == nil {
		p.code = b.typed.Issue(p.req.ID)
	}
	code := p.code
	b.mu.Unlock()
	if b.OnPending != nil {
		b.OnPending(p.req, m, code)
	}
	return core.ApprovalDecision{}, false
}

// reissue gives p a fresh typed code and tells the UI.
func (b *Broker) reissue(p *pendingReq) {
	b.typed.Forget(p.req.ID)
	b.mu.Lock()
	p.code = b.typed.Issue(p.req.ID)
	method, code := p.method, p.code
	b.mu.Unlock()
	if b.OnPending != nil {
		b.OnPending(p.req, method, code)
	}
}
