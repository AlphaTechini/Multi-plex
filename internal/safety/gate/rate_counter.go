package gate

import (
	"sync"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
)

// RateCounter counts approvals per session over a sliding hour. It is the
// approval fatigue metric of docs/04: if a workspace regularly sees more than
// a few approvals per hour the rules are wrong, and the UI should say so. The
// zero value is ready to use and safe for concurrent use.
type RateCounter struct {
	mu     sync.Mutex
	events map[core.SessionID][]time.Time
}

// Window is the sliding window of the counter.
const Window = time.Hour

// Add records one approval request for session at time t.
func (c *RateCounter) Add(session core.SessionID, t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.events == nil {
		c.events = map[core.SessionID][]time.Time{}
	}
	c.events[session] = append(c.prune(c.events[session], t), t)
}

// PerHour is the number of approvals for session in the hour up to now.
func (c *RateCounter) PerHour(session core.SessionID, now time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.countLocked(session, now)
}

// TotalPerHour is the number of approvals across all sessions in the hour up
// to now.
func (c *RateCounter) TotalPerHour(now time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for s := range c.events {
		n += c.countLocked(s, now)
	}
	return n
}

// Forget drops a session's history, for example when the session ends.
func (c *RateCounter) Forget(session core.SessionID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.events, session)
}

func (c *RateCounter) countLocked(session core.SessionID, now time.Time) int {
	kept := c.prune(c.events[session], now)
	if len(kept) == 0 {
		delete(c.events, session)
		return 0
	}
	c.events[session] = kept
	n := 0
	for _, t := range kept {
		if !t.After(now) {
			n++
		}
	}
	return n
}

// prune drops events older than the window before ref, in place.
func (c *RateCounter) prune(ts []time.Time, ref time.Time) []time.Time {
	cut := ref.Add(-Window)
	kept := ts[:0]
	for _, t := range ts {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	return kept
}
