package daemon

import (
	"context"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// hCommentPut saves a review comment, assigning an ID when it has none.
func (d *Daemon) hCommentPut(ctx context.Context, _ *conn, c core.Comment) (any, error) {
	if _, err := d.getSession(c.SessionID); err != nil {
		return nil, err
	}
	if c.Path == "" || c.NewStart <= 0 {
		return nil, badRequest("a comment needs a path and a line")
	}
	if c.NewEnd < c.NewStart {
		c.NewEnd = c.NewStart
	}
	if c.ID == "" {
		c.ID = core.NewID("cmt")
	}
	if c.Status == "" {
		c.Status = core.CommentDraft
	}
	if c.At.IsZero() {
		c.At = d.now()
	}
	if err := d.store.PutComment(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// hCommentList lists a session's comments.
func (d *Daemon) hCommentList(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	if _, err := d.getSession(p.SessionID); err != nil {
		return nil, err
	}
	list, err := d.store.ListComments(ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []core.Comment{}
	}
	return list, nil
}

// hCommentDelete deletes a comment.
func (d *Daemon) hCommentDelete(ctx context.Context, _ *conn, p protocol.IDRef) (any, error) {
	if err := d.store.DeleteComment(ctx, p.ID); err != nil {
		return nil, err
	}
	return protocol.Empty{}, nil
}

// hCommentSend formats the session's queued comments as one message
// (docs/07-diff-viewer.md "Sending to the agent"), types it into the agent
// and marks the comments sent.
func (d *Daemon) hCommentSend(ctx context.Context, _ *conn, p protocol.SessionRef) (any, error) {
	s, err := d.getSession(p.SessionID)
	if err != nil {
		return nil, err
	}
	all, err := d.store.ListComments(ctx, p.SessionID)
	if err != nil {
		return nil, err
	}
	var queued []core.Comment
	var items []diff.CommentItem
	for _, c := range all {
		if c.Status != core.CommentQueued {
			continue
		}
		queued = append(queued, c)
		items = append(items, diff.CommentItem{
			Path: c.Path, Start: c.NewStart, End: c.NewEnd,
			Language: c.Language, Snippet: c.Snippet, Body: c.Body,
		})
	}
	if len(queued) == 0 {
		return protocol.CommentSendResult{}, nil
	}
	info := s.snap()
	msg := diff.FormatComments(info.Slug, items)
	a, err := adapterFor(info)
	if err != nil {
		return nil, err
	}
	if err := d.writeAgent(s, a.TypeMessage(msg)); err != nil {
		return nil, err
	}
	for _, c := range queued {
		c.Status = core.CommentSent
		if err := d.store.PutComment(ctx, c); err != nil {
			d.log.Error("mark comment sent", "comment", c.ID, "err", err)
		}
	}
	return protocol.CommentSendResult{Sent: len(queued), Message: msg}, nil
}
