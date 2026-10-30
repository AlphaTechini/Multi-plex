package clientbackend

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

func sref(id core.SessionID) protocol.SessionRef { return protocol.SessionRef{SessionID: id} }

// Sessions lists the sessions of a workspace (all workspaces when ws is "").
func (b *Backend) Sessions(ctx context.Context, ws core.WorkspaceID) ([]core.Session, error) {
	var out []core.Session
	err := b.call(ctx, protocol.MethodSessionList, protocol.SessionListParams{WorkspaceID: ws}, &out)
	return out, err
}

// Session returns everything the Overview needs for one session.
func (b *Backend) Session(ctx context.Context, id core.SessionID) (protocol.SessionDetail, error) {
	var out protocol.SessionDetail
	err := b.call(ctx, protocol.MethodSessionGet, sref(id), &out)
	return out, err
}

// CreateSession creates a session with its worktree and agent.
func (b *Backend) CreateSession(ctx context.Context, p protocol.SessionCreateParams) (core.Session, error) {
	var out core.Session
	err := b.call(ctx, protocol.MethodSessionCreate, p, &out)
	return out, err
}

// StopSession kills the agent and keeps the worktree.
func (b *Backend) StopSession(ctx context.Context, id core.SessionID) error {
	return b.call(ctx, protocol.MethodSessionStop, sref(id), nil)
}

// RestartSession starts the agent again.
func (b *Backend) RestartSession(ctx context.Context, id core.SessionID) (core.Session, error) {
	var out core.Session
	err := b.call(ctx, protocol.MethodSessionRestart, sref(id), &out)
	return out, err
}

// RemoveSession removes the session, its worktree and branch.
func (b *Backend) RemoveSession(ctx context.Context, id core.SessionID) error {
	return b.call(ctx, protocol.MethodSessionRemove, sref(id), nil)
}

// MarkSeen clears the session's unseen flag.
func (b *Backend) MarkSeen(ctx context.Context, id core.SessionID) error {
	return b.call(ctx, protocol.MethodSessionSeen, sref(id), nil)
}

// SetFollow turns follow mode on or off for a session.
func (b *Backend) SetFollow(ctx context.Context, id core.SessionID, on bool) error {
	return b.call(ctx, protocol.MethodSessionFollow, protocol.SessionFollowParams{SessionID: id, On: on}, nil)
}

// Message types text into the session's agent (steering).
func (b *Backend) Message(ctx context.Context, id core.SessionID, text string, interrupt bool) error {
	return b.call(ctx, protocol.MethodSessionMessage,
		protocol.SessionMessageParams{SessionID: id, Text: text, Interrupt: interrupt}, nil)
}

// Interrupt interrupts the agent's current turn.
func (b *Backend) Interrupt(ctx context.Context, id core.SessionID) error {
	return b.call(ctx, protocol.MethodSessionInterrupt, sref(id), nil)
}
