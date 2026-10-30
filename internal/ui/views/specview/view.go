// Package specview is the Specs main-view mode (docs/09): the workspace's
// specs with their progress, and for the selected spec its requirements,
// design and tasks documents, rendered read-only as styled markdown-lite.
package specview

import (
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/kit"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// loadState tracks one fetch.
type loadState uint8

const (
	notLoaded loadState = iota
	loading
	loaded
	failed
)

// doc is a spec's three documents and what has been rendered from them.
type doc struct {
	state loadState
	err   string
	spec  protocol.SpecDoc
	// Rendered lines, per document, valid for the width they were made at.
	lines [3][]mdLine
	width int
}

// model is the Specs view. Update and Render run on the Bubble Tea goroutine;
// Render fills the rendered-markdown cache and records layout state, so it is
// a pointer type.
type model struct {
	th *theme.Theme
	be backend.Backend
	ws core.Workspace

	focused bool

	list    loadState
	listErr string
	specs   []protocol.SpecInfo
	sel     int
	docs    map[string]*doc
	kind    docKind
	scroll  [3]map[string]int // per document kind, per slug
	detail  bool              // single-pane layouts: the document is shown, not the list
	want    string            // slug to select once the list arrives

	// Render state.
	listW   int
	twoPane bool
	bodyH   int
	total   int // lines in the document on screen
	tabX    [3][2]int
	listY   int
	now     func() time.Time
}

// New returns the Specs view.
func New(env view.Env) view.View {
	m := &model{
		th: env.Theme, be: env.Backend, ws: env.Workspace,
		docs: map[string]*doc{},
		now:  time.Now,
	}
	for i := range m.scroll {
		m.scroll[i] = map[string]int{}
	}
	return m
}

// Tab returns "Specs" with no badge.
func (m *model) Tab() (string, int) { return "Specs", 0 }

// Animating is always false: the view has no animation.
func (m *model) Animating() bool { return false }

// Capturing is always false: the view has no text input.
func (m *model) Capturing() bool { return false }

// Hints returns the key hints for the status bar.
func (m *model) Hints() []kit.Hint {
	if len(m.specs) == 0 {
		return nil
	}
	h := []kit.Hint{{Keys: "up down", Label: "spec"}, {Keys: "left right", Label: "document"}, {Keys: "j k", Label: "scroll"}}
	if !m.twoPane && m.detail {
		h = []kit.Hint{{Keys: "left right", Label: "document"}, {Keys: "j k", Label: "scroll"}, {Keys: "esc", Label: "specs"}}
	} else if !m.twoPane {
		h = []kit.Hint{{Keys: "up down", Label: "spec"}, {Keys: "enter", Label: "open"}}
	}
	return h
}

// current returns the selected spec.
func (m *model) current() (protocol.SpecInfo, bool) {
	if m.sel < 0 || m.sel >= len(m.specs) {
		return protocol.SpecInfo{}, false
	}
	return m.specs[m.sel], true
}
