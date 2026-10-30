package codeview

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// rpcTimeout bounds every backend call the view makes.
const rpcTimeout = 5 * time.Second

// openReq says how to place the file once it has been read.
type openReq struct {
	seq   int
	sess  core.SessionID
	path  string
	line  int // 1-based line to put the cursor on; 0 keeps the position
	first int // 1-based fresh range, 0 for none
	last  int
	keep  bool // reload of the file already open: keep cursor and scroll
}

// fileMsg is the result of readCmd.
type fileMsg struct {
	req openReq
	res protocol.FileReadResult
	err error
}

// changesMsg is the result of changesCmd.
type changesMsg struct {
	sess    core.SessionID
	changes []vcs.FileChange
	err     error
}

// readCmd reads one file of the session's worktree.
func readCmd(be backend.Backend, req openReq) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()
		res, err := be.ReadFile(ctx, req.sess, req.path, "")
		return fileMsg{req: req, res: res, err: err}
	}
}

// changesCmd lists the files the session changed.
func changesCmd(be backend.Backend, id core.SessionID) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()
		d, err := be.Diff(ctx, id)
		return changesMsg{sess: id, changes: d.Changes, err: err}
	}
}

// followCmd turns the daemon's follow mode on or off for a session. The
// result message restores the local state when the call fails.
func followCmd(be backend.Backend, id core.SessionID, on bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		defer cancel()
		if err := be.SetFollow(ctx, id, on); err != nil {
			return followErrMsg{sess: id, on: on, err: err}
		}
		return nil
	}
}

// followErrMsg reports a failed SetFollow.
type followErrMsg struct {
	sess core.SessionID
	on   bool
	err  error
}

// normPath turns a path from a link or a picker into the slash-separated,
// workspace-relative form the backend expects.
func normPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "\\", "/")
	for strings.HasPrefix(p, "./") {
		p = p[2:]
	}
	return p
}
