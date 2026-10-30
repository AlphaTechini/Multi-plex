package daemon

import (
	"context"
	"encoding/json"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Limits of session_wait, in seconds.
const (
	defaultWaitSeconds = 30 * 60
	maxWaitSeconds     = 4 * 60 * 60
)

// settled reports whether a session has stopped for now: done, blocked,
// idle or in error.
func settled(st core.SessionState) bool {
	return st == core.StateDone || st == core.StateBlocked || st == core.StateIdle || st == core.StateError
}

// mcpSessionWait blocks until every named session has settled, or the timeout
// passes, or the call is cancelled.
func (d *Daemon) mcpSessionWait(ctx context.Context, s *session, raw json.RawMessage) (string, error) {
	var a protocol.SessionWaitArgs
	if err := decodeArgs(raw, &a); err != nil {
		return "", err
	}
	if len(a.Sessions) == 0 {
		return "", badRequest("name at least one session to wait for")
	}
	targets, err := d.resolveSessions(s, a.Sessions)
	if err != nil {
		return "", err
	}
	for _, t := range targets {
		if t == s {
			return "", badRequest("a session cannot wait for itself")
		}
	}
	secs := a.TimeoutSeconds
	if secs <= 0 {
		secs = defaultWaitSeconds
	}
	secs = min(secs, maxWaitSeconds)
	timer := time.NewTimer(time.Duration(secs) * time.Second)
	defer timer.Stop()

	timedOut := false
	for {
		changed := d.stateChanged() // taken before the check, so no change is missed
		all := true
		for _, t := range targets {
			if !settled(t.snap().State) {
				all = false
				break
			}
		}
		if all {
			break
		}
		select {
		case <-changed:
		case <-timer.C:
			timedOut = true
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if timedOut {
			break
		}
	}

	briefs := make([]sessionBrief, len(targets))
	for i, t := range targets {
		briefs[i] = d.briefOf(ctx, t)
	}
	return toJSONText(map[string]any{"timed_out": timedOut, "sessions": briefs})
}
