package diffview

import (
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// content is the working-tree content of one file.
type content struct {
	lines  []string
	binary bool
	// hash is the hash of the file entry the content was read for.
	hash uint64
}

// sessionMem is what the view remembers about one session while it is
// running: reviewed marks and the last position in each file.
type sessionMem struct {
	rev       *reviewState
	positions map[string]posState
	lastPath  string
}

// gapRange is a range of new-side lines the user expanded.
type gapRange struct{ from, to int }

// Model is the Diff view. It implements view.View.
type Model struct {
	env view.Env
	t   *theme.Theme
	hl  highlight.Highlighter
	now func() time.Time

	session *core.Session
	// gen is the session epoch: results of commands started for another
	// session (or before a reset) are dropped.
	gen     int
	focused bool
	w, h    int

	loaded, loading         bool
	debouncing, reloadAgain bool
	loadErr                 string
	base, head              string

	files    []*fileEntry
	filesVer int
	fi       int // index of the current file, -1 when none
	fv       *fileView

	contents map[string]*content
	order    []string          // contents in the order they were read
	reading  map[string]uint64 // file reads in flight
	tok      map[string]*fileTokens
	hlBusy   bool

	expanded  map[string][]gapRange // gaps expanded between hunks, by path
	innerExp  map[string]map[int]bool
	appliedGp map[*fileEntry]bool
	forceHuge map[string]bool
	fresh     map[string]map[lineID]struct{}

	mems map[core.SessionID]*sessionMem
	mem  *sessionMem
	rev  *reviewState

	comments []core.Comment
	cmp      *composer
	saves    []core.Comment // relocated comments to store

	// UI state.
	railHidden bool
	overlay    bool
	overlayCur int
	railTop    int
	sbs        bool
	hx         int
	visual     bool // range selection mode (V)
	dragging   bool // left button held after a click on a line

	anim animState

	railCache railCache
	eofCache  eofCache
	blank     map[int]string
}

var _ view.View = (*Model)(nil)

// Tab returns the tab label and the number of changed files.
func (m *Model) Tab() (string, int) { return "Diff", len(m.files) }

// Hints returns the status-bar key hints for the current state.
func (m *Model) Hints() []kit.Hint {
	if m.cmp != nil {
		return []kit.Hint{
			{Keys: "enter", Label: "queue"},
			{Keys: "ctrl+s", Label: "send now"},
			{Keys: "ctrl+d", Label: "save draft"},
			{Keys: "esc", Label: "cancel"},
		}
	}
	if m.overlay {
		return []kit.Hint{{Keys: "j k", Label: "move"}, {Keys: "enter", Label: "open"}, {Keys: "esc", Label: "close"}}
	}
	hints := []kit.Hint{
		{Keys: "[", Label: "prev file"},
		{Keys: "]", Label: "next file"},
		{Keys: "n", Label: "next change"},
		{Keys: "c", Label: "comment"},
		{Keys: "v", Label: "reviewed"},
		{Keys: "f", Label: "files"},
	}
	if m.sbsAllowed() {
		hints = append(hints, kit.Hint{Keys: "s", Label: "split"})
	}
	return hints
}

// Animating reports whether the view needs animation ticks.
func (m *Model) Animating() bool {
	return m.t.Animated() && (m.anim.switching || m.anim.freshOn)
}

// Capturing reports whether the view wants raw keys: while a comment is
// being typed.
func (m *Model) Capturing() bool { return m.cmp != nil }

// sbsAllowed reports whether the terminal is wide enough for side-by-side.
func (m *Model) sbsAllowed() bool { return m.w >= sbsMinWidth }

// railShown reports whether the file rail is drawn at the current width.
func (m *Model) railShown() bool { return m.w >= railMinWidth && !m.railHidden }

// diffWidth is the width of the diff area.
func (m *Model) diffWidth() int {
	if m.railShown() {
		return m.w - railWidth - 1
	}
	return m.w
}
