package diffview

import (
	"slices"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/diff"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// loadDiffCmd fetches the session's diff, parses it and computes word-level
// highlights, all off the render loop.
func (m *Model) loadDiffCmd() tea.Cmd {
	m.loading = true
	be, id, gen := m.env.Backend, m.session.ID, m.gen
	return func() tea.Msg {
		ctx, cancel := ctxTimeout()
		defer cancel()
		res, err := be.Diff(ctx, id)
		if err != nil {
			return diffLoadedMsg{gen: gen, err: err}
		}
		files, perr := diff.Parse(res.Raw)
		if len(files) == 0 && perr != nil && res.Raw != "" {
			return diffLoadedMsg{gen: gen, err: perr}
		}
		for i := range files {
			diff.ApplyWordDiff(&files[i])
		}
		return diffLoadedMsg{gen: gen, res: res, files: files}
	}
}

// onDiffLoaded installs a freshly loaded diff.
func (m *Model) onDiffLoaded(msg diffLoadedMsg) tea.Cmd {
	if msg.gen != m.gen {
		return nil
	}
	m.loading = false
	var cmds []tea.Cmd
	if msg.err != nil {
		m.loadErr = msg.err.Error()
		if m.loaded {
			cmds = append(cmds, toastErr("Could not refresh the diff", msg.err))
		}
	} else {
		m.loadErr = ""
		first := !m.loaded
		m.loaded = true
		m.base, m.head = msg.res.Base, msg.res.Head
		cmds = append(cmds, m.applyDiff(msg, first))
		if !first {
			cmds = append(cmds, m.commentsCmd())
		}
	}
	if m.reloadAgain {
		m.reloadAgain = false
		cmds = append(cmds, m.scheduleRefresh())
	}
	return tea.Batch(cmds...)
}

// stubFile builds a file entry for a change the diff text does not cover
// (for example a file too large for the daemon to include).
func stubFile(ch vcs.FileChange) diff.File {
	f := diff.File{Status: ch.Status, Binary: ch.Binary, Added: ch.Added, Deleted: ch.Deleted}
	switch ch.Status {
	case vcs.Deleted:
		f.OldPath = ch.Path
	case vcs.Added:
		f.NewPath = ch.Path
	default:
		f.NewPath, f.OldPath = ch.Path, ch.OldPath
		if f.OldPath == "" {
			f.OldPath = ch.Path
		}
	}
	return f
}

// applyDiff replaces the file list with a new diff while keeping the current
// file and position, marking hunks that are new since the last refresh (not
// on the first load).
func (m *Model) applyDiff(msg diffLoadedMsg, first bool) tea.Cmd {
	next := make([]*fileEntry, 0, len(msg.files))
	seen := make(map[string]bool, len(msg.files))
	for i := range msg.files {
		e := newEntry(msg.files[i])
		if seen[e.path] {
			continue
		}
		seen[e.path] = true
		next = append(next, e)
	}
	for _, ch := range msg.res.Changes {
		if !seen[ch.Path] {
			seen[ch.Path] = true
			next = append(next, newEntry(stubFile(ch)))
		}
	}
	sortEntries(next)

	oldBy := make(map[string]*fileEntry, len(m.files))
	for _, e := range m.files {
		oldBy[e.path] = e
	}
	changed := len(next) != len(m.files)
	freshAny := false
	for i, e := range next {
		old := oldBy[e.path]
		if old != nil && old.hash == e.hash {
			next[i] = old // identical: keep its expanded regions and content
			if !changed && i < len(m.files) && m.files[i] != old {
				changed = true
			}
			continue
		}
		changed = true
		m.reapplyInner(e)
		if !first && len(e.file.Hunks) > 0 {
			if set := freshLines(e, old); len(set) > 0 {
				m.fresh[e.path] = set
				freshAny = true
			}
		}
	}
	if !changed {
		return nil
	}
	// Forget the bookkeeping of entries that are gone.
	for _, e := range m.files {
		if !slices.Contains(next, e) {
			delete(m.appliedGp, e)
		}
	}

	oldPath, oldIdx := "", m.fi
	if m.fi >= 0 && m.fi < len(m.files) {
		oldPath = m.files[m.fi].path
	}
	pos := m.capturePos()
	m.files = next
	m.filesVer++
	if freshAny {
		m.startFresh()
	}
	if len(next) == 0 {
		m.leaveFile()
		m.fi, m.fv = -1, nil
		return nil
	}

	ni := slices.IndexFunc(next, func(e *fileEntry) bool { return e.path == oldPath })
	switch {
	case m.fv == nil:
		start := 0
		if m.mem != nil && m.mem.lastPath != "" {
			if i := slices.IndexFunc(next, func(e *fileEntry) bool { return e.path == m.mem.lastPath }); i >= 0 {
				start = i
			}
		}
		return m.openFile(start, false)
	case ni < 0:
		// The current file left the diff: go to the nearest one.
		near := sort.Search(len(next), func(i int) bool {
			a, b := next[i], m.fv.e
			if a.dir != b.dir {
				return a.dir >= b.dir
			}
			return a.name >= b.name
		})
		if near >= len(next) {
			near = len(next) - 1
		}
		_ = oldIdx
		return m.openFile(near, true)
	}
	m.fi = ni
	cur := next[ni]
	if cur == m.fv.e {
		return nil // the current file did not change
	}
	// The current file changed: rebuild it in place at the same position.
	m.fv.e = cur
	m.fv.tok = m.tokens(cur.path)
	m.rebuild()
	m.restorePos(pos)
	m.fv.gen++
	return m.readContentCmd(cur)
}

// freshLines returns the lines of the hunks of e that did not exist in old.
func freshLines(e, old *fileEntry) map[lineID]struct{} {
	known := map[uint64]bool{}
	if old != nil {
		for _, h := range old.hunkHashes {
			known[h] = true
		}
	}
	var set map[lineID]struct{}
	for hi := range e.file.Hunks {
		if known[e.hunkHashes[hi]] {
			continue
		}
		if set == nil {
			set = map[lineID]struct{}{}
		}
		for li := range e.file.Hunks[hi].Lines {
			set[idOf(&e.file.Hunks[hi].Lines[li])] = struct{}{}
		}
	}
	return set
}

// onComments installs the comments loaded from the backend.
func (m *Model) onComments(msg commentsMsg) tea.Cmd {
	if msg.gen != m.gen || msg.err != nil {
		return nil
	}
	if slices.Equal(m.comments, msg.list) {
		return nil
	}
	m.comments = msg.list
	return m.refreshComments()
}

// refreshComments re-places the comments of the current file.
func (m *Model) refreshComments() tea.Cmd {
	if m.fv == nil {
		return nil
	}
	pos := m.capturePos()
	m.rebuild()
	m.restorePos(pos)
	return m.flushSaves()
}

// flushSaves stores comments whose lines moved.
func (m *Model) flushSaves() tea.Cmd {
	if len(m.saves) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(m.saves))
	for _, c := range m.saves {
		cmds = append(cmds, m.putCommentCmd(c, false, true))
	}
	m.saves = nil
	return tea.Batch(cmds...)
}

// onCommentSaved handles a stored comment (and a send, when asked for).
func (m *Model) onCommentSaved(msg commentSavedMsg) tea.Cmd {
	if msg.gen != m.gen {
		return nil
	}
	if msg.err != nil {
		return toastErr("Could not save the comment", msg.err)
	}
	m.upsertSaved(msg.c)
	pos := m.capturePos()
	m.rebuild()
	m.restorePos(pos)
	if msg.quiet {
		return nil
	}
	name := m.sessionName()
	switch {
	case msg.sent != nil && msg.sendErr == nil:
		return tea.Batch(toastCmd(kit.ToneSuccess, sentTitle(msg.sent.Sent), "to "+name), m.commentsCmd())
	case msg.sendErr != nil:
		return toastErr("Comment saved, but sending failed", msg.sendErr)
	case msg.c.Status == core.CommentDraft:
		return toastCmd(kit.ToneInfo, "Draft saved", "Only you see drafts. Edit one with e to queue it.")
	}
	return toastCmd(kit.ToneSuccess, "Comment queued for "+name,
		itoa(m.queuedCount())+" queued "+m.t.Glyphs.Dot+" sent at the agent's next turn")
}

// upsertSaved stores the backend's version of a comment, replacing the
// optimistic copy (which has no ID yet) that was shown while saving.
func (m *Model) upsertSaved(c core.Comment) {
	if c.ID == "" {
		return
	}
	for i := range m.comments {
		if m.comments[i].ID == c.ID {
			m.comments[i] = c
			return
		}
	}
	for i := range m.comments {
		o := m.comments[i]
		if o.ID == "" && o.Path == c.Path && o.NewStart == c.NewStart && o.NewEnd == c.NewEnd && o.Body == c.Body {
			m.comments[i] = c
			return
		}
	}
	m.comments = append(m.comments, c)
}

// onCommentDeleted drops a deleted comment.
func (m *Model) onCommentDeleted(msg commentDeletedMsg) tea.Cmd {
	if msg.gen != m.gen {
		return nil
	}
	if msg.err != nil {
		return toastErr("Could not delete the comment", msg.err)
	}
	return nil
}

// onSent handles queued comments sent to the agent.
func (m *Model) onSent(msg sentMsg) tea.Cmd {
	if msg.gen != m.gen {
		return nil
	}
	if msg.err != nil {
		return toastErr("Could not send the comments", msg.err)
	}
	return tea.Batch(toastCmd(kit.ToneSuccess, sentTitle(msg.res.Sent), "to "+m.sessionName()), m.commentsCmd())
}

func sentTitle(n int) string {
	if n == 1 {
		return "1 comment sent"
	}
	return itoa(n) + " comments sent"
}

// sessionName is the session's name for toasts.
func (m *Model) sessionName() string {
	if m.session == nil {
		return ""
	}
	if m.session.Slug != "" {
		return m.session.Slug
	}
	return string(m.session.ID)
}
