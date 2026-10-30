package core

import "time"

// Checkpoint is a commit object of a worktree captured before a risky step,
// stored under refs/multiplex/checkpoints/<session>/<n>.
type Checkpoint struct {
	SessionID SessionID `json:"session_id"`
	N         int       `json:"n"`
	Ref       string    `json:"ref"`
	Commit    string    `json:"commit"`
	Reason    string    `json:"reason"`
	At        time.Time `json:"at"`
}
