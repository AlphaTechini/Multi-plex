package protocol

// MethodSessionPush asks the daemon to push a session's branch (the push
// broker of docs/04-safety.md Layer 3). It blocks until a human approves or
// denies the push.
const MethodSessionPush = "session.push" // SessionRef → PushResult

// PushResult reports a completed push.
type PushResult struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	Output string `json:"output"` // git's own output
}
