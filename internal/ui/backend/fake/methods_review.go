package fake

import (
	"context"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

// Diff returns the session's multi-file unified diff.
func (b *Backend) Diff(ctx context.Context, id core.SessionID) (protocol.DiffResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Diff")
	s := b.session(id)
	if s == nil {
		return protocol.DiffResult{}, notFound("session")
	}
	return protocol.DiffResult{
		Base: "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678", Head: "e4f5a6b7c8d9e0f1a2b3c4d5e6f708192a3b4c5d",
		Raw: rawDiff(s.Slug), Changes: changesOf(s.Slug),
	}, ctxErr(ctx)
}

// ReadFile returns a file of the fake worktree.
func (b *Backend) ReadFile(ctx context.Context, id core.SessionID, path, rev string) (protocol.FileReadResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("ReadFile")
	s := b.session(id)
	if s == nil {
		return protocol.FileReadResult{}, notFound("session")
	}
	content, ok := fileContent(s.Slug, path, rev)
	if !ok {
		return protocol.FileReadResult{}, notFound("file " + path)
	}
	return protocol.FileReadResult{Path: path, Content: content}, ctxErr(ctx)
}

// PutComment stores a comment, assigning its ID and time.
func (b *Backend) PutComment(ctx context.Context, c core.Comment) (core.Comment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("PutComment")
	if c.ID == "" {
		c.ID = b.nextID("cmt")
	}
	if c.Status == "" {
		c.Status = core.CommentDraft
	}
	c.At = b.now()
	list := b.comments[c.SessionID]
	for i := range list {
		if list[i].ID == c.ID {
			list[i] = c
			return c, ctxErr(ctx)
		}
	}
	b.comments[c.SessionID] = append(list, c)
	return c, ctxErr(ctx)
}

// Comments lists a session's comments.
func (b *Backend) Comments(ctx context.Context, id core.SessionID) ([]core.Comment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Comments")
	return append([]core.Comment(nil), b.comments[id]...), ctxErr(ctx)
}

// DeleteComment removes a comment.
func (b *Backend) DeleteComment(ctx context.Context, commentID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("DeleteComment")
	for sid, list := range b.comments {
		for i := range list {
			if list[i].ID == commentID {
				b.comments[sid] = append(list[:i], list[i+1:]...)
				return ctxErr(ctx)
			}
		}
	}
	return ctxErr(ctx)
}

// SendComments formats the queued comments with diff.FormatComments, marks
// them sent and types the message into the agent's terminal.
func (b *Backend) SendComments(ctx context.Context, id core.SessionID) (protocol.CommentSendResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("SendComments")
	s := b.session(id)
	if s == nil {
		return protocol.CommentSendResult{}, notFound("session")
	}
	var items []diff.CommentItem
	list := b.comments[id]
	for i := range list {
		if list[i].Status != core.CommentQueued {
			continue
		}
		items = append(items, diff.CommentItem{
			Path: list[i].Path, Start: list[i].NewStart, End: list[i].NewEnd,
			Language: list[i].Language, Snippet: list[i].Snippet, Body: list[i].Body,
		})
		list[i].Status = core.CommentSent
	}
	if len(items) == 0 {
		return protocol.CommentSendResult{}, ctxErr(ctx)
	}
	msg := diff.FormatComments(s.Slug, items)
	if p := b.panes[s.AgentPane]; p != nil {
		p.entries = append(p.entries, entry{kind: entUser, text: "Review comments (" + itoa(len(items)) + ") sent from the diff"})
		b.pushFrameLocked(p, false)
	}
	return protocol.CommentSendResult{Sent: len(items), Message: msg}, ctxErr(ctx)
}

// Notes lists a session's insight cards.
func (b *Backend) Notes(ctx context.Context, id core.SessionID) ([]core.Note, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("Notes")
	return append([]core.Note(nil), b.notes[id]...), ctxErr(ctx)
}

// ActOnNote dismisses a card, or sends the ask/steer text to the agent.
func (b *Backend) ActOnNote(ctx context.Context, p protocol.NoteActParams) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rec("ActOnNote")
	for sid, list := range b.notes {
		for i := range list {
			if list[i].ID != p.NoteID {
				continue
			}
			if p.Action == core.NoteDismiss {
				list[i].Dismissed = true
				b.emit(protocol.TopicNoteDismissed, list[i])
				return ctxErr(ctx)
			}
			if s := b.session(sid); s != nil && p.Action != core.NoteAsk {
				b.userMessageLocked(s, p.Text)
			}
			return ctxErr(ctx)
		}
	}
	return notFound("note")
}

// addNoteLocked appends a card and announces it.
func (b *Backend) addNoteLocked(id core.SessionID, sev core.Severity, title, body string, refs ...string) core.Note {
	n := core.Note{
		ID: b.nextID("note"), SessionID: id, Number: len(b.notes[id]) + 1, Title: title, Body: body,
		Severity: sev, Refs: refs, At: b.now().Truncate(time.Second),
	}
	b.notes[id] = append(b.notes[id], n)
	b.emit(protocol.TopicNote, n)
	return n
}
