package core

import "time"

// EditEvent says an agent edited a file range. Follow mode jumps to it.
type EditEvent struct {
	SessionID SessionID `json:"session_id"`
	Path      string    `json:"path"` // relative to the worktree root
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	At        time.Time `json:"at"`
}
