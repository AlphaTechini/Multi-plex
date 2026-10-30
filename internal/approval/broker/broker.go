package broker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/approval/typedcode"
	"github.com/alphatechini/multi-plex/internal/core"
)

// Defaults for a Broker.
const (
	DefaultTimeout     = 10 * time.Minute
	DefaultMaxFailures = 3
	// MethodNone is recorded for decisions made without any verification.
	MethodNone = "none"
)

// Errors returned by Request and Respond.
var (
	ErrUnknownRequest = errors.New("approval: no such pending request")
	ErrDuplicate      = errors.New("approval: request id already pending")
	ErrBusy           = errors.New("approval: a verification is already running for this request")
	ErrNoMethod       = errors.New("approval: no approval method available")
)

// Pending describes a request that is waiting, for UI clients that attach late.
type Pending struct {
	Request core.ApprovalRequest `json:"request"`
	Method  string               `json:"method"`
	Code    string               `json:"code,omitempty"` // typed-code only; hand to the UI, never log
}

// Broker coordinates approval requests. It is safe for concurrent use. Set
// the exported fields before the first Request.
type Broker struct {
	// OnPending is called when a request is registered, and again if its
	// method or code changes. For method "typed-code" the code must be shown
	// in the UI only. May be nil.
	OnPending func(req core.ApprovalRequest, method string, code string)
	// OnResolved is called with every final decision. May be nil.
	OnResolved func(req core.ApprovalRequest, dec core.ApprovalDecision)
	// Timeout bounds a whole request; zero means DefaultTimeout.
	Timeout time.Duration
	// MaxFailures is the number of failed verifications before the request is
	// denied; zero means DefaultMaxFailures. Cancelled prompts do not count.
	MaxFailures int

	verifiers []approval.Verifier
	typed     *typedcode.Verifier
	audit     func(core.ApprovalRequest, core.ApprovalDecision)
	now       func() time.Time

	mu   sync.Mutex
	reqs map[string]*pendingReq
}

// pendingReq is the broker's state for one request; guarded by Broker.mu.
type pendingReq struct {
	req       core.ApprovalRequest
	ctx       context.Context
	verifier  approval.Verifier // nil for the typed-code method
	method    string
	code      string
	failures  int
	verifying bool
	tried     map[string]bool // methods that reported themselves unavailable
	done      bool
	decision  core.ApprovalDecision
	result    chan core.ApprovalDecision
}

// New returns a Broker that tries verifiers in order and falls back to typed
// when none is available (typed may be nil to disable the fallback). audit, if
// non-nil, receives every final decision.
func New(verifiers []approval.Verifier, typed *typedcode.Verifier, audit func(core.ApprovalRequest, core.ApprovalDecision)) *Broker {
	return &Broker{
		verifiers: verifiers,
		typed:     typed,
		audit:     audit,
		now:       time.Now,
		reqs:      make(map[string]*pendingReq),
	}
}

func (b *Broker) timeout() time.Duration {
	if b.Timeout > 0 {
		return b.Timeout
	}
	return DefaultTimeout
}

func (b *Broker) maxFailures() int {
	if b.MaxFailures > 0 {
		return b.MaxFailures
	}
	return DefaultMaxFailures
}

// Pending returns the requests still waiting, oldest first.
func (b *Broker) Pending() []core.ApprovalRequest {
	b.mu.Lock()
	out := make([]core.ApprovalRequest, 0, len(b.reqs))
	for _, p := range b.reqs {
		out = append(out, p.req)
	}
	b.mu.Unlock()
	sortRequests(out)
	return out
}

// PendingDetails is Pending plus each request's method and typed code, for a UI
// client that connects after OnPending already fired.
func (b *Broker) PendingDetails() []Pending {
	b.mu.Lock()
	out := make([]Pending, 0, len(b.reqs))
	for _, p := range b.reqs {
		out = append(out, Pending{Request: p.req, Method: p.method, Code: p.code})
	}
	b.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return before(out[i].Request, out[j].Request) })
	return out
}

func sortRequests(rs []core.ApprovalRequest) {
	sort.Slice(rs, func(i, j int) bool { return before(rs[i], rs[j]) })
}

func before(a, b core.ApprovalRequest) bool {
	if !a.At.Equal(b.At) {
		return a.At.Before(b.At)
	}
	return a.ID < b.ID
}

// choose picks the first available verifier not in tried, else the typed-code
// fallback. method is "" when nothing can be used.
func (b *Broker) choose(ctx context.Context, tried map[string]bool) (v approval.Verifier, method string) {
	for _, c := range b.verifiers {
		if !tried[c.Name()] && c.Available(ctx) {
			return c, c.Name()
		}
	}
	if b.typed != nil && !tried[b.typed.Name()] {
		return nil, b.typed.Name()
	}
	return nil, ""
}

// finalize records the final decision once. It returns the decision in force
// (the earlier one if the request was already decided).
func (b *Broker) finalize(p *pendingReq, outcome core.ApprovalOutcome, message, method string) core.ApprovalDecision {
	b.mu.Lock()
	if p.done {
		d := p.decision
		b.mu.Unlock()
		return d
	}
	p.done = true
	p.decision = core.ApprovalDecision{
		RequestID: p.req.ID,
		Outcome:   outcome,
		Message:   message,
		Method:    method,
		At:        b.now(),
	}
	d := p.decision
	delete(b.reqs, p.req.ID)
	b.mu.Unlock()

	if b.typed != nil {
		b.typed.Forget(p.req.ID)
	}
	if b.audit != nil {
		b.audit(p.req, d)
	}
	if b.OnResolved != nil {
		b.OnResolved(p.req, d)
	}
	p.result <- d // buffered, written once
	return d
}

// describe builds the short text shown by OS prompts; the UI shows full details.
func describe(req core.ApprovalRequest) string {
	what := req.Command
	if what == "" && len(req.Paths) > 0 {
		what = strings.Join(req.Paths, ", ")
	}
	if what == "" {
		what = req.Tool
	}
	s := fmt.Sprintf("Multiplex: approve %s", req.Kind)
	if what != "" {
		s += ": " + what
	}
	return s
}
