// Package approval is the neutral layer for human-present approvals
// (docs/04-safety.md Layer 5). The daemon's broker asks a Verifier; each
// method (Windows Hello, polkit, a typed one-time code shown only in the
// UI) is one implementation in its own subpackage.
package approval

import (
	"context"
	"errors"

	"github.com/alphatechini/multi-plex/internal/core"
)

// Errors a Verifier may return.
var (
	ErrUnavailable = errors.New("approval: method not available on this machine")
	ErrCancelled   = errors.New("approval: cancelled by user")
	ErrTimeout     = errors.New("approval: timed out")
	ErrFailed      = errors.New("approval: verification failed")
)

// Verifier proves that a human is present and consents.
type Verifier interface {
	// Name is recorded in the audit log, e.g. "windows-hello".
	Name() string
	// Available reports whether this method works on this machine.
	Available(ctx context.Context) bool
	// Verify shows the method's prompt (message describes the action) and
	// returns nil only if the human approved.
	Verify(ctx context.Context, req core.ApprovalRequest, message string) error
}
