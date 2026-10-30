package diffview

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// undoQueue remembers the comment queued last, so "u" can take it back while
// its toast is still on screen.
type undoQueue struct {
	id    string
	until time.Time
}

// queuedHints are the actions the "Comment queued" toast offers.
var queuedHints = []kit.Hint{{Keys: "ctrl+s", Label: "send now"}, {Keys: "u", Label: "undo"}}

// armUndo makes comment id undoable for as long as the toast lives.
func (m *Model) armUndo(id string) {
	if id == "" {
		m.undo = undoQueue{}
		return
	}
	m.undo = undoQueue{id: id, until: m.now().Add(m.t.Motion.ToastDuration)}
}

// undoQueued deletes the comment queued last, if its toast has not expired
// and the comment is still waiting (not sent yet).
func (m *Model) undoQueued() tea.Cmd {
	u := m.undo
	m.undo = undoQueue{}
	if u.id == "" || !m.now().Before(u.until) {
		return nil
	}
	found := false
	for i := range m.comments {
		if m.comments[i].ID == u.id {
			if m.comments[i].Status != core.CommentQueued {
				return nil // already sent: too late
			}
			m.comments = append(m.comments[:i], m.comments[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	pos := m.capturePos()
	m.rebuild()
	m.restorePos(pos)
	return tea.Batch(m.deleteCommentCmd(u.id), toastCmd(kit.ToneInfo, "Comment removed", "The queued comment was taken back."))
}

// toastHintsCmd returns a command that asks the shell for a toast with
// action hints.
func toastHintsCmd(tone kit.Tone, title, body string, hints []kit.Hint) tea.Cmd {
	return func() tea.Msg { return view.ToastMsg{Tone: tone, Title: title, Body: body, Hints: hints} }
}
