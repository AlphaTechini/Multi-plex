package core

// SessionState is where a session is in its lifecycle.
//
// starting → working ⇄ blocked → done → idle, plus error.
type SessionState string

// Session states. See docs/01-architecture.md.
const (
	StateStarting SessionState = "starting"
	StateWorking  SessionState = "working"
	StateBlocked  SessionState = "blocked"
	StateDone     SessionState = "done"
	StateIdle     SessionState = "idle"
	StateError    SessionState = "error"
)

// NeedsUser reports whether the state asks for the user's attention.
func (s SessionState) NeedsUser() bool { return s == StateBlocked || s == StateError }

// Active reports whether an agent process is expected to be running.
func (s SessionState) Active() bool {
	return s == StateStarting || s == StateWorking || s == StateBlocked
}
