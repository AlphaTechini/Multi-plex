package typedcode

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alphatechini/multi-plex/internal/approval"
	"github.com/alphatechini/multi-plex/internal/core"
)

const (
	// Alphabet has no 0, O, 1, I or L, so a code read off a screen is unambiguous.
	Alphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	// CodeLength is the number of characters in a code.
	CodeLength = 6
	// MaxAttempts is how many wrong codes are allowed before the request fails.
	MaxAttempts = 3
	// DefaultTTL is how long an issued code stays valid.
	DefaultTTL = 5 * time.Minute
)

// Verifier issues and checks one-time codes. It is safe for concurrent use.
type Verifier struct {
	ttl time.Duration

	mu      sync.Mutex
	entries map[string]*entry
}

type entry struct {
	code     string
	expires  time.Time
	attempts int
	done     chan struct{} // closed when finished
	err      error         // valid once done is closed; nil means verified
}

// New returns a Verifier with the default five-minute code lifetime.
func New() *Verifier {
	return &Verifier{ttl: DefaultTTL, entries: make(map[string]*entry)}
}

// Name implements approval.Verifier.
func (v *Verifier) Name() string { return "typed-code" }

// Available implements approval.Verifier. The fallback always works.
func (v *Verifier) Available(context.Context) bool { return true }

// Issue returns the code for requestID, creating it if needed. Calling it again
// for a live request returns the same code and does not reset the attempts.
func (v *Verifier) Issue(requestID string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sweepLocked()
	return v.issueLocked(requestID).code
}

func (v *Verifier) issueLocked(id string) *entry {
	if e, ok := v.entries[id]; ok && (e.finished() || time.Now().Before(e.expires)) {
		return e
	}
	e := &entry{code: newCode(), expires: time.Now().Add(v.ttl), done: make(chan struct{})}
	v.entries[id] = e
	return e
}

func (e *entry) finished() bool {
	select {
	case <-e.done:
		return true
	default:
		return false
	}
}

// finish records the outcome once.
func (e *entry) finish(err error) {
	if !e.finished() {
		e.err = err
		close(e.done)
	}
}

// sweepLocked drops entries that expired a while ago (finished entries stay
// until then so a late Verify still sees the outcome).
func (v *Verifier) sweepLocked() {
	now := time.Now()
	for id, e := range v.entries {
		if now.After(e.expires.Add(time.Minute)) {
			delete(v.entries, id)
		}
	}
}

// Forget drops any code for requestID, for example once the request is decided.
func (v *Verifier) Forget(requestID string) {
	v.mu.Lock()
	delete(v.entries, requestID)
	v.mu.Unlock()
}

// Submit checks a code typed by the human. It returns nil if the code is right
// (which also releases the blocked Verify), ErrFailed for a wrong code or when
// no code is pending, and ErrTimeout if the code expired. After MaxAttempts
// wrong codes the request is failed for good.
func (v *Verifier) Submit(requestID, code string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	e, ok := v.entries[requestID]
	if !ok || e.finished() {
		return fmt.Errorf("%w: no pending code for this request", approval.ErrFailed)
	}
	if time.Now().After(e.expires) {
		e.finish(approval.ErrTimeout)
		return approval.ErrTimeout
	}
	e.attempts++
	if subtle.ConstantTimeCompare([]byte(normalize(code)), []byte(e.code)) == 1 {
		e.finish(nil)
		return nil
	}
	if e.attempts >= MaxAttempts {
		e.finish(approval.ErrFailed)
		return fmt.Errorf("%w: too many wrong codes", approval.ErrFailed)
	}
	return fmt.Errorf("%w: wrong code (%d attempts left)", approval.ErrFailed, MaxAttempts-e.attempts)
}

// Verify implements approval.Verifier. It blocks until Submit has accepted the
// code for req.ID, attempts run out (ErrFailed), the code expires (ErrTimeout)
// or ctx ends (ErrTimeout on deadline, otherwise the context's error). The code
// must already have been shown through Issue; if not, one is created here.
func (v *Verifier) Verify(ctx context.Context, req core.ApprovalRequest, _ string) error {
	v.mu.Lock()
	e := v.issueLocked(req.ID)
	v.mu.Unlock()
	defer func() {
		v.mu.Lock()
		if v.entries[req.ID] == e {
			delete(v.entries, req.ID)
		}
		v.mu.Unlock()
	}()

	timer := time.NewTimer(time.Until(e.expires))
	defer timer.Stop()
	select {
	case <-e.done:
		return e.err
	case <-timer.C:
		v.mu.Lock()
		e.finish(approval.ErrTimeout)
		v.mu.Unlock()
		return e.err
	case <-ctx.Done():
		v.mu.Lock()
		e.finish(ctx.Err())
		v.mu.Unlock()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return approval.ErrTimeout
		}
		return ctx.Err()
	}
}

// normalize upper-cases and strips spaces and dashes, so "ab3 k7x" is accepted.
func normalize(s string) string {
	s = strings.ToUpper(s)
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return -1
		}
		return r
	}, s)
}

// newCode returns CodeLength random characters from Alphabet, without modulo bias.
func newCode() string {
	out := make([]byte, 0, CodeLength)
	var buf [16]byte
	limit := byte(256 - 256%len(Alphabet))
	for len(out) < CodeLength {
		if _, err := rand.Read(buf[:]); err != nil {
			panic("typedcode: crypto/rand failed: " + err.Error())
		}
		for _, b := range buf {
			if b < limit && len(out) < CodeLength {
				out = append(out, Alphabet[int(b)%len(Alphabet)])
			}
		}
	}
	return string(out)
}
