package diffview

import (
	"time"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/highlight"
	"github.com/alphatechini/multi-plex/internal/highlight/chromahl"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// New returns the Diff view. It starts without a session; the shell selects
// one with view.SessionMsg.
func New(env view.Env) view.View {
	return newModel(env, chromahl.New(), time.Now)
}

// newModel builds the view with an explicit highlighter and clock (tests use
// fakes).
func newModel(env view.Env, hl highlight.Highlighter, now func() time.Time) *Model {
	m := &Model{
		env: env, t: env.Theme, hl: hl, now: now,
		w: 120, h: 40, fi: -1,
		mems: map[core.SessionID]*sessionMem{},
	}
	m.initMaps()
	return m
}

// initMaps (re)creates the per-session maps.
func (m *Model) initMaps() {
	m.contents = map[string]*content{}
	m.order = nil
	m.reading = map[string]uint64{}
	m.tok = map[string]*fileTokens{}
	m.expanded = map[string][]gapRange{}
	m.innerExp = map[string]map[int]bool{}
	m.appliedGp = map[*fileEntry]bool{}
	m.forceHuge = map[string]bool{}
	m.fresh = map[string]map[lineID]struct{}{}
}
