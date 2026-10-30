package core

// Conflict is a predicted merge conflict between a session's branch and
// another branch (the base or another session's).
type Conflict struct {
	SessionID    SessionID `json:"session_id"`
	OtherBranch  string    `json:"other_branch"`
	OtherSession SessionID `json:"other_session,omitempty"`
	Files        []string  `json:"files"`
}
