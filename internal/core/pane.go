package core

// PaneKind says what runs in a pane.
type PaneKind string

// Pane kinds.
const (
	PaneAgent PaneKind = "agent"
	PaneShell PaneKind = "shell"
)

// Pane is a terminal hosted by the daemon: a PTY plus its emulator state.
type Pane struct {
	ID        PaneID    `json:"id"`
	SessionID SessionID `json:"session_id"`
	Kind      PaneKind  `json:"kind"`
	Cols      int       `json:"cols"`
	Rows      int       `json:"rows"`
	Exited    bool      `json:"exited"`
	ExitCode  int       `json:"exit_code"`
}
