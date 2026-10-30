package clientbackend

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Diff returns the session's diff against its base.
func (b *Backend) Diff(ctx context.Context, id core.SessionID) (protocol.DiffResult, error) {
	var out protocol.DiffResult
	err := b.call(ctx, protocol.MethodDiffGet, sref(id), &out)
	return out, err
}

// ReadFile reads a file of the session's worktree at a revision ("" = working tree).
func (b *Backend) ReadFile(ctx context.Context, id core.SessionID, path, rev string) (protocol.FileReadResult, error) {
	var out protocol.FileReadResult
	err := b.call(ctx, protocol.MethodFileRead, protocol.FileReadParams{SessionID: id, Path: path, Rev: rev}, &out)
	return out, err
}

// PutComment stores a review comment and returns it with its ID assigned.
func (b *Backend) PutComment(ctx context.Context, c core.Comment) (core.Comment, error) {
	var out core.Comment
	err := b.call(ctx, protocol.MethodCommentPut, c, &out)
	return out, err
}

// Comments lists the session's review comments.
func (b *Backend) Comments(ctx context.Context, id core.SessionID) ([]core.Comment, error) {
	var out []core.Comment
	err := b.call(ctx, protocol.MethodCommentList, sref(id), &out)
	return out, err
}

// DeleteComment deletes a review comment.
func (b *Backend) DeleteComment(ctx context.Context, commentID string) error {
	return b.call(ctx, protocol.MethodCommentDelete, protocol.IDRef{ID: commentID}, nil)
}

// SendComments sends the session's queued comments to its agent.
func (b *Backend) SendComments(ctx context.Context, id core.SessionID) (protocol.CommentSendResult, error) {
	var out protocol.CommentSendResult
	err := b.call(ctx, protocol.MethodCommentSend, sref(id), &out)
	return out, err
}

// Notes lists the session's insight cards.
func (b *Backend) Notes(ctx context.Context, id core.SessionID) ([]core.Note, error) {
	var out []core.Note
	err := b.call(ctx, protocol.MethodNoteList, sref(id), &out)
	return out, err
}

// ActOnNote asks, steers or dismisses an insight card.
func (b *Backend) ActOnNote(ctx context.Context, p protocol.NoteActParams) error {
	return b.call(ctx, protocol.MethodNoteAct, p, nil)
}
