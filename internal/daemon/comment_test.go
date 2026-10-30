package daemon

import (
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
)

func TestCommentsRoundTripAndSend(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "review")
	h.ev.waitScreen(t, sess.AgentPane, "helper-agent-ready")

	// An empty queue sends nothing.
	var none protocol.CommentSendResult
	h.mustCall(protocol.MethodCommentSend, protocol.SessionRef{SessionID: sess.ID}, &none)
	if none.Sent != 0 || none.Message != "" {
		t.Errorf("empty send = %+v", none)
	}

	var c1, c2, c3 core.Comment
	h.mustCall(protocol.MethodCommentPut, core.Comment{
		SessionID: sess.ID, Path: "src/auth/session.go", NewStart: 12, NewEnd: 30, Language: "go",
		Snippet: "func refresh() {}", Body: "cache the token instead", Status: core.CommentQueued,
	}, &c1)
	h.mustCall(protocol.MethodCommentPut, core.Comment{
		SessionID: sess.ID, Path: "src/auth/session_test.go", NewStart: 88, Snippet: "t.Skip()", Body: "do not skip", Status: core.CommentQueued,
	}, &c2)
	h.mustCall(protocol.MethodCommentPut, core.Comment{SessionID: sess.ID, Path: "README.md", NewStart: 1, Body: "draft only", Status: core.CommentDraft}, &c3)
	if c1.ID == "" || c1.At.IsZero() {
		t.Errorf("comment.put did not fill ID/time: %+v", c1)
	}

	var list []core.Comment
	h.mustCall(protocol.MethodCommentList, protocol.SessionRef{SessionID: sess.ID}, &list)
	if len(list) != 3 {
		t.Fatalf("comments = %d", len(list))
	}

	var sent protocol.CommentSendResult
	h.mustCall(protocol.MethodCommentSend, protocol.SessionRef{SessionID: sess.ID}, &sent)
	if sent.Sent != 2 {
		t.Fatalf("sent = %d, want 2 (drafts stay)", sent.Sent)
	}
	for _, want := range []string{
		"Review comments (session: review):",
		"1. src/auth/session.go:L12-L30",
		"```go", "func refresh() {}",
		"Comment: cache the token instead",
		"2. src/auth/session_test.go:L88",
	} {
		if !strings.Contains(sent.Message, want) {
			t.Errorf("message lacks %q:\n%s", want, sent.Message)
		}
	}
	if strings.Contains(sent.Message, "draft only") {
		t.Error("a draft comment was sent")
	}
	// The message was typed into the agent pane (the helper is not a TUI, so the
	// paste markers show up raw, and the first lines scroll off the 20-row screen).
	h.ev.waitScreen(t, sess.AgentPane, "Comment: do not skip")

	h.mustCall(protocol.MethodCommentList, protocol.SessionRef{SessionID: sess.ID}, &list)
	states := map[string]core.CommentStatus{}
	for _, c := range list {
		states[c.Path] = c.Status
	}
	if states["src/auth/session.go"] != core.CommentSent || states["README.md"] != core.CommentDraft {
		t.Errorf("statuses = %v", states)
	}
	// Sending again sends nothing new.
	h.mustCall(protocol.MethodCommentSend, protocol.SessionRef{SessionID: sess.ID}, &none)
	if none.Sent != 0 {
		t.Errorf("second send = %+v", none)
	}

	h.mustCall(protocol.MethodCommentDelete, protocol.IDRef{ID: c3.ID}, nil)
	h.mustCall(protocol.MethodCommentList, protocol.SessionRef{SessionID: sess.ID}, &list)
	if len(list) != 2 {
		t.Errorf("after delete: %d comments", len(list))
	}
	if err := h.call(protocol.MethodCommentPut, core.Comment{SessionID: "ses_nope", Path: "a", NewStart: 1}, nil); err == nil {
		t.Error("comment on an unknown session accepted")
	}
	if err := h.call(protocol.MethodCommentPut, core.Comment{SessionID: sess.ID}, nil); err == nil {
		t.Error("comment without a path accepted")
	}
}

func TestNoteActions(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "cards")
	h.ev.waitScreen(t, sess.AgentPane, "helper-agent-ready")
	h.mcp(sess.ID, protocol.ToolNoteToUser, protocol.NoteToUserArgs{Title: "API change", Body: "Callers break."})
	n := decode[core.Note](t, h.ev.wait(t, protocol.TopicNote, nil))

	// Steer types "Re card #N (title): text".
	h.mustCall(protocol.MethodNoteAct, protocol.NoteActParams{NoteID: n.ID, Action: core.NoteSteer, Text: "keep the old signature"}, nil)
	h.ev.waitScreen(t, sess.AgentPane, "Re card #1 (API change): keep the old signature")

	// Interrupt-steer: the interrupt byte then the message.
	h.mustCall(protocol.MethodNoteAct, protocol.NoteActParams{NoteID: n.ID, Action: core.NoteInterruptSteer, Text: "stop and rethink"}, nil)
	h.ev.waitScreen(t, sess.AgentPane, "Re card #1 (API change): stop and rethink")

	// Ask uses the adapter's side question; the helper (Claude Code based)
	// types "/btw ..." with the card context.
	h.mustCall(protocol.MethodNoteAct, protocol.NoteActParams{NoteID: n.ID, Action: core.NoteAsk, Text: "why?"}, nil)
	screen := h.ev.waitScreen(t, sess.AgentPane, "/btw Re: card #1 API change: Callers break. why?")
	_ = screen

	if err := h.call(protocol.MethodNoteAct, protocol.NoteActParams{NoteID: n.ID, Action: core.NoteAsk}, nil); err == nil {
		t.Error("ask without a question accepted")
	}
	if err := h.call(protocol.MethodNoteAct, protocol.NoteActParams{NoteID: "note_nope", Action: core.NoteDismiss}, nil); err == nil {
		t.Error("unknown note accepted")
	}
	h.mustCall(protocol.MethodNoteAct, protocol.NoteActParams{NoteID: n.ID, Action: core.NoteDismiss}, nil)
	gone := decode[core.Note](t, h.ev.wait(t, protocol.TopicNoteDismissed, nil))
	if gone.ID != n.ID || gone.Number != 1 || !gone.Dismissed {
		t.Errorf("note.dismissed = %+v", gone)
	}
	var notes []core.Note
	h.mustCall(protocol.MethodNoteList, protocol.SessionRef{SessionID: sess.ID}, &notes)
	if len(notes) != 1 || !notes[0].Dismissed {
		t.Errorf("notes after dismiss = %+v", notes)
	}
}

func TestSessionMessageAndInterrupt(t *testing.T) {
	h := newHarness(t)
	sess := h.newSession("helper", "steer")
	h.ev.waitScreen(t, sess.AgentPane, "helper-agent-ready")
	h.mustCall(protocol.MethodSessionMessage, protocol.SessionMessageParams{SessionID: sess.ID, Text: "please add tests"}, nil)
	h.ev.waitScreen(t, sess.AgentPane, "please add tests")
	h.mustCall(protocol.MethodSessionMessage, protocol.SessionMessageParams{SessionID: sess.ID, Text: "actually, stop", Interrupt: true}, nil)
	h.ev.waitScreen(t, sess.AgentPane, "actually, stop")
	h.mustCall(protocol.MethodSessionInterrupt, protocol.SessionRef{SessionID: sess.ID}, nil)
	if err := h.call(protocol.MethodSessionMessage, protocol.SessionMessageParams{SessionID: sess.ID}, nil); err == nil {
		t.Error("empty message accepted")
	}
}
