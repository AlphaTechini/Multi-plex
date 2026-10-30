package core

import "time"

// Workspace is one repository the user opened. Its Root is the base for
// every relative path shown to the user or sent to an agent.
type Workspace struct {
	ID        WorkspaceID `json:"id"`
	Root      string      `json:"root"`
	Name      string      `json:"name"`
	CreatedAt time.Time   `json:"created_at"`
}
