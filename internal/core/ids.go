package core

import (
	"crypto/rand"
	"encoding/hex"
)

// WorkspaceID identifies a workspace (one repository root).
type WorkspaceID string

// SessionID identifies one agent session.
type SessionID string

// PaneID identifies one terminal pane hosted by the daemon.
type PaneID string

// NewID returns a random identifier with the given prefix, for example
// "ses_1f3a9c0b2d4e6f70".
func NewID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("core: crypto/rand failed: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
