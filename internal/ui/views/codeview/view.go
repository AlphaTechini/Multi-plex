// Package codeview is the Code main-view mode (docs/06): a read-only,
// syntax-highlighted viewer for the selected session's worktree, a go-to-file
// picker over the session's changed files, and follow mode, which opens each
// file the agent edits and scrolls to the edited lines.
//
// Follow mode is view-only: nothing in this package ever sends text to the
// agent.
package codeview

import (
	"strings"
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/highlight/chromahl"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/syntax"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// maxFileBytes is the largest file the view opens; bigger ones show a note.
const maxFileBytes = 5 << 20

// defaultRows is the viewport height assumed before the first Render.
const defaultRows = 30

// freshLines is a range of lines the agent just wrote, highlighted briefly.
type freshLines struct {
	first, last int // 0-based, inclusive
	at          time.Time
	on          bool
}

// model is the Code view. Update and Render run on the Bubble Tea goroutine;
// Render records the viewport height and hit rows the handlers use, so it is
// a pointer type.
type model struct {
	th *theme.Theme
	be backend.Backend
	ws core.Workspace
	hl highlight.Highlighter

	now   func() time.Time
	clock time.Time

	frame   int
	focused bool

	sess    *core.Session
	changes []vcs.FileChange

	chLoading, chDirty bool

	// The open file. src is nil while nothing is open or the file could not
	// be read; note then says why.
	path    string
	src     *syntax.Source
	note    string
	loading string // path being read, for the banner
	reqSeq  int

	cursor, top int
	fresh       freshLines
	rows        int // viewport rows from the last Render

	follow bool
	paused bool
	last   *core.EditEvent // newest edit seen, for resuming

	picker *picker

	seqs map[sgrKey]string // cached escape sequences, see sgr
}

// New returns the Code view.
func New(env view.Env) view.View {
	m := &model{
		th:   env.Theme,
		be:   env.Backend,
		ws:   env.Workspace,
		hl:   chromahl.New(),
		now:  time.Now,
		rows: defaultRows,
		seqs: map[sgrKey]string{},
	}
	m.clock = m.now()
	return m
}

// Tab returns "Code" with no badge.
func (m *model) Tab() (string, int) { return "Code", 0 }

// Animating reports whether the new-lines highlight is decaying.
func (m *model) Animating() bool {
	return m.fresh.on && m.th.Animated() && m.clock.Sub(m.fresh.at) < m.th.Motion.HunkDecay
}

// Capturing reports whether the go-to-file box wants raw keys.
func (m *model) Capturing() bool { return m.picker != nil }

// Hints returns the key hints for the status bar.
func (m *model) Hints() []kit.Hint {
	if m.picker != nil {
		return []kit.Hint{{Keys: "up down", Label: "move"}, {Keys: "enter", Label: "open"}, {Keys: "esc", Label: "close"}}
	}
	f := kit.Hint{Keys: "f", Label: "follow"}
	switch {
	case m.follow && m.paused:
		f.Label = "resume following"
	case m.follow:
		f.Label = "stop following"
	}
	return []kit.Hint{f, {Keys: "p", Label: "go to file"}, {Keys: "[ ]", Label: "prev/next change"}, {Keys: "e", Label: "editor"}}
}

// dirOf and baseOf split a slash path.
func dirOf(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i+1]
	}
	return ""
}

func baseOf(p string) string { return p[len(dirOf(p)):] }
