package diffview

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
)

// composer is the comment being written or edited. The text lives in a
// bubbles textarea (editing keys, paste, word movement); the view draws it
// inside a kit.CommentBox.
type composer struct {
	ta textarea.Model
	// edit is the comment being edited (zero value for a new comment).
	edit   core.Comment
	editID string
	// anchor is where a new comment attaches.
	anchor anchorInfo
	// unit is the unit the box hangs under.
	unit int
}

// composeHints are the box's actions (docs/16 mockup).
var composeHints = []kit.Hint{
	{Keys: "enter", Label: "queue"},
	{Keys: "ctrl+s", Label: "send now"},
	{Keys: "esc", Label: "cancel"},
}

// startCompose opens the comment box under the selection.
func (m *Model) startCompose() tea.Cmd {
	fv := m.fv
	if fv == nil || len(fv.units) == 0 {
		return nil
	}
	a, ok := m.selectionAnchor()
	if !ok {
		return toastCmd(kit.ToneInfo, "Select a line to comment on", "Move onto a changed or unchanged code line first.")
	}
	m.cmp = &composer{ta: newTextarea(), anchor: a, unit: a.lastUnit}
	m.layout()
	m.keepCursorVisible()
	return nil
}

// startEdit opens an existing comment for editing.
func (m *Model) startEdit(p *placedComment) tea.Cmd {
	ta := newTextarea()
	ta.SetValue(p.c.Body)
	m.cmp = &composer{ta: ta, edit: p.c, editID: p.c.ID, unit: p.unit}
	if p.unit < 0 {
		m.cmp.unit = -1
	}
	m.layout()
	return nil
}

// newTextarea returns the editing buffer.
func newTextarea() textarea.Model {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.SetWidth(60)
	ta.SetHeight(8)
	ta.Focus()
	return ta
}

// composerLines renders the comment box being edited.
func (m *Model) composerLines() []string {
	fv := m.fv
	cm := m.cmp
	rng := cm.anchor.label
	status := "draft"
	if cm.editID != "" {
		rng = m.rangeLabel(cm.edit.NewStart, cm.edit.NewEnd)
		if cm.edit.OldStart > 0 && cm.edit.Snippet != "" && !strings.Contains(rng, "L") {
			rng = m.rangeLabel(cm.edit.OldStart, cm.edit.OldEnd)
		}
	}
	return kit.Lines(kit.CommentBox(m.t, kit.CommentBoxOptions{
		Width: fv.w, Indent: kit.DiffGutterWidth(fv.nw) - 2,
		Author: "You", Status: status, Range: rng, Body: cm.ta.Value(),
		Hints: composeHints, Focused: true,
	}))
}

// composeKey handles a key while the comment box is open.
func (m *Model) composeKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.cmp = nil
		m.layout()
		return nil
	case "enter":
		return m.saveCompose(core.CommentQueued, false)
	case "ctrl+s":
		return m.saveCompose(core.CommentQueued, true)
	case "ctrl+d":
		return m.saveCompose(core.CommentDraft, false)
	case "shift+enter", "alt+enter", "ctrl+j":
		m.cmp.ta.InsertString("\n")
		m.layout()
		return nil
	}
	m.cmp.ta.SetWidth(m.composeWidth())
	// The textarea's blink commands are dropped: the box draws its own
	// static cursor, and nothing may tick while the view is idle.
	m.cmp.ta, _ = m.cmp.ta.Update(msg)
	m.layout()
	return nil
}

// composeWidth is the width of the text inside the comment box.
func (m *Model) composeWidth() int {
	return max(m.fv.w-(kit.DiffGutterWidth(m.fv.nw)-2)-4, 10)
}

// composePaste inserts pasted text into the comment box.
func (m *Model) composePaste(msg tea.PasteMsg) tea.Cmd {
	m.cmp.ta, _ = m.cmp.ta.Update(msg)
	m.layout()
	return nil
}

// saveCompose stores the comment with a status; sendNow also sends every
// queued comment to the agent.
func (m *Model) saveCompose(status core.CommentStatus, sendNow bool) tea.Cmd {
	cm := m.cmp
	body := strings.TrimRight(cm.ta.Value(), " \t\r\n")
	if strings.TrimSpace(body) == "" {
		m.cmp = nil
		m.layout()
		return nil
	}
	var c core.Comment
	if cm.editID != "" {
		c = cm.edit
	} else {
		a := cm.anchor
		c = core.Comment{
			SessionID: m.session.ID, Path: a.path,
			NewStart: a.newStart, NewEnd: a.newEnd, OldStart: a.oldStart, OldEnd: a.oldEnd,
			BaseCommit: m.base, HeadCommit: m.head,
			Snippet: a.snippet, Context: a.context, Language: m.language(a.path),
		}
	}
	c.Body, c.Status, c.At = body, status, m.now()
	m.cmp = nil
	// Show it at once; the backend's answer replaces it.
	m.upsertComment(c)
	m.rebuild()
	m.keepCursorVisible()
	return m.putCommentCmd(c, sendNow, false)
}

// language returns the fence language of a path.
func (m *Model) language(path string) string {
	if m.hl == nil {
		return ""
	}
	return m.hl.Language(path)
}

// upsertComment adds or replaces a comment in the local list.
func (m *Model) upsertComment(c core.Comment) {
	for i := range m.comments {
		if c.ID != "" && m.comments[i].ID == c.ID {
			m.comments[i] = c
			return
		}
	}
	m.comments = append(m.comments, c)
}

// putCommentCmd stores a comment through the backend (and sends the queued
// comments when sendNow is set).
func (m *Model) putCommentCmd(c core.Comment, sendNow, quiet bool) tea.Cmd {
	be, id, gen := m.env.Backend, m.session.ID, m.gen
	return func() tea.Msg {
		ctx, cancel := ctxTimeout()
		defer cancel()
		saved, err := be.PutComment(ctx, c)
		msg := commentSavedMsg{gen: gen, c: saved, err: err, quiet: quiet}
		if err == nil && sendNow {
			res, serr := be.SendComments(ctx, id)
			msg.sent, msg.sendErr = &res, serr
		}
		return msg
	}
}

// sendQueuedCmd sends every queued comment to the agent.
func (m *Model) sendQueuedCmd() tea.Cmd {
	if m.session == nil {
		return nil
	}
	if m.queuedCount() == 0 {
		return toastCmd(kit.ToneInfo, "No queued comments", "Press enter in a comment box to queue it.")
	}
	be, id, gen := m.env.Backend, m.session.ID, m.gen
	return func() tea.Msg {
		ctx, cancel := ctxTimeout()
		defer cancel()
		res, err := be.SendComments(ctx, id)
		return sentMsg{gen: gen, res: res, err: err}
	}
}

// deleteCommentCmd removes a comment.
func (m *Model) deleteCommentCmd(id string) tea.Cmd {
	be, gen := m.env.Backend, m.gen
	return func() tea.Msg {
		ctx, cancel := ctxTimeout()
		defer cancel()
		return commentDeletedMsg{gen: gen, id: id, err: be.DeleteComment(ctx, id)}
	}
}

// commentCmd loads the session's comments.
func (m *Model) commentsCmd() tea.Cmd {
	be, id, gen := m.env.Backend, m.session.ID, m.gen
	return func() tea.Msg {
		ctx, cancel := ctxTimeout()
		defer cancel()
		list, err := be.Comments(ctx, id)
		return commentsMsg{gen: gen, list: list, err: err}
	}
}
