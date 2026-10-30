package app

import (
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// kitState maps a core session state to the UI state.
func kitState(s core.SessionState) kit.State {
	switch s {
	case core.StateStarting:
		return kit.StateStarting
	case core.StateWorking:
		return kit.StateWorking
	case core.StateBlocked:
		return kit.StateBlocked
	case core.StateDone:
		return kit.StateDone
	case core.StateError:
		return kit.StateError
	default:
		return kit.StateIdle
	}
}

// sessionDetail is the second sidebar line: role and age while working, what
// the session needs when it is blocked, else the daemon's own detail.
func sessionDetail(s core.Session, now time.Time) string {
	switch {
	case s.StateDetail != "":
		return s.StateDetail
	case s.State == core.StateBlocked:
		return "needs approval"
	case s.State == core.StateStarting:
		return "starting"
	case s.State == core.StateError:
		return "error"
	case s.State == core.StateDone:
		return "done"
	}
	return s.Role + " · " + age(now.Sub(s.CreatedAt))
}

// age formats a duration as the compact "14m" / "2h" / "3d" used in the UI.
func age(d time.Duration) string {
	switch {
	case d < 0:
		return "0s"
	case d < time.Minute:
		return itoa(int(d/time.Second)) + "s"
	case d < time.Hour:
		return itoa(int(d/time.Minute)) + "m"
	case d < 48*time.Hour:
		return itoa(int(d/time.Hour)) + "h"
	}
	return itoa(int(d/(24*time.Hour))) + "d"
}

// agentLabel is the short CLI name shown in the agent panel's frame.
func agentLabel(agent string) string {
	if a, _, ok := strings.Cut(agent, "-"); ok && a != "" {
		return a
	}
	return agent
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
