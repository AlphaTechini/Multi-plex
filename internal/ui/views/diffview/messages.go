package diffview

import (
	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// Messages produced by this view's commands. Every one carries the session
// epoch (gen) it was started for, so results that arrive after the session
// changed are dropped.

// diffLoadedMsg carries a parsed diff.
type diffLoadedMsg struct {
	gen   int
	res   protocol.DiffResult
	files []diff.File
	err   error
}

// contentMsg carries the working-tree content of one file.
type contentMsg struct {
	gen    int
	path   string
	hash   uint64 // hash of the file entry the content was requested for
	lines  []string
	binary bool
	err    error
}

// commentsMsg carries the session's comments.
type commentsMsg struct {
	gen  int
	list []core.Comment
	err  error
}

// commentSavedMsg reports a saved comment (and a send, when asked for).
type commentSavedMsg struct {
	gen     int
	c       core.Comment
	err     error
	sent    *protocol.CommentSendResult
	sendErr error
	// quiet suppresses the toast (relocation saves).
	quiet bool
}

// commentDeletedMsg reports a deleted comment.
type commentDeletedMsg struct {
	gen int
	id  string
	err error
}

// sentMsg reports queued comments sent to the agent.
type sentMsg struct {
	gen int
	res protocol.CommentSendResult
	err error
}

// refreshDueMsg fires when the refresh debounce window ends.
type refreshDueMsg struct{ gen int }

// toastCmd returns a command that asks the shell for a toast.
func toastCmd(tone kit.Tone, title, body string) tea.Cmd {
	return func() tea.Msg { return view.ToastMsg{Tone: tone, Title: title, Body: body} }
}
