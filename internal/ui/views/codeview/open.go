package codeview

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/syntax"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// open starts reading path and, when it arrives, puts the cursor on line
// (1-based; 0 keeps the position when it is the file already shown, else the
// top). first and last mark freshly written lines (0 for none). Global files
// ("~/...") live outside the worktree and are not read: the view says so.
func (m *model) open(path string, line, first, last int, keep bool) tea.Cmd {
	path = normPath(path)
	if m.sess == nil || path == "" {
		return nil
	}
	m.reqSeq++
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		m.path, m.src, m.loading = path, nil, ""
		m.note = "global"
		m.cursor, m.top = 0, 0
		return nil
	}
	m.loading = path
	if m.src == nil {
		m.path, m.note = path, "opening"
	}
	return readCmd(m.be, openReq{seq: m.reqSeq, sess: m.sess.ID, path: path, line: line, first: first, last: last, keep: keep})
}

// onFile applies a finished read.
func (m *model) onFile(msg fileMsg) tea.Cmd {
	req := msg.req
	if m.sess == nil || req.sess != m.sess.ID || req.seq != m.reqSeq {
		return nil
	}
	m.loading = ""
	same := m.path == req.path
	m.path = req.path
	switch {
	case msg.err != nil:
		m.src, m.note = nil, "error:"+clean(msg.err.Error(), true)
		m.cursor, m.top = 0, 0
		return nil
	case msg.res.Binary:
		m.src, m.note = nil, "binary"
		m.cursor, m.top = 0, 0
		return nil
	case len(msg.res.Content) > maxFileBytes:
		m.src, m.note = nil, "large"
		m.cursor, m.top = 0, 0
		return nil
	}
	m.note = ""
	prev := m.src
	m.src = syntax.NewSource(m.hl, req.path, msg.res.Content)
	m.src.Inherit(prev)
	n := m.src.NumLines()
	switch {
	case req.line > 0:
		m.cursor = min(req.line-1, max(n-1, 0))
		m.top = max(m.cursor-m.rows/3, 0)
	case !same || !req.keep:
		m.cursor, m.top = 0, 0
	}
	m.clampScroll()
	if req.first > 0 {
		last := max(req.last, req.first)
		m.fresh = freshLines{first: req.first - 1, last: last - 1, at: m.clock, on: true}
		if n > 0 && req.line == 0 {
			m.cursor = min(req.first-1, n-1)
		}
		m.showRange(req.first-1, last-1)
	}
	return m.request()
}

// request asks for syntax highlighting of what is on screen.
func (m *model) request() tea.Cmd {
	if m.src == nil {
		return nil
	}
	return m.src.Request(m.top, m.top+m.rows)
}

// editorToast tells the user where the cursor is, for opening it in their
// editor (the launch itself is out of scope).
func (m *model) editorToast() tea.Cmd {
	if m.path == "" {
		return nil
	}
	body := m.path
	if m.src != nil {
		body += ":" + itoa(m.cursor+1)
	}
	return func() tea.Msg {
		return view.ToastMsg{Tone: kit.ToneInfo, Title: "Open in your editor", Body: body}
	}
}
