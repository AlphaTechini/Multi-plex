package app

import (
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/ui/pane"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// sidebarKey is everything the sidebar's rendering depends on.
type sidebarKey struct {
	th             *theme.Theme
	w, h           int
	focused        bool
	frame          int
	ver            uint64
	cursor, scroll int
	cur            core.SessionID
}

func (m *Model) cachedSidebar() string {
	k := sidebarKey{m.th, m.lay.sidebarW, m.lay.bodyH, m.focus == regionSidebar, m.frame, m.ver, m.sb.cursor, m.sb.scroll, m.cur}
	if out, ok := m.sbCache.get(k, m.noCache); ok {
		return out
	}
	return m.sbCache.put(k, m.renderSidebar())
}

// mainKey is everything the main panel's rendering depends on. The views are
// pure functions of their state, which changes only through Update, so
// mainVer (bumped on every message to the active view) covers them.
type mainKey struct {
	th       *theme.Theme
	w, h     int
	focused  bool
	mode     int
	ver      uint64
	loaded   bool
	welcome  bool
	frame    int
	meta     string
	loadErr  string
	hasViews int
}

func (m *Model) cachedMain() string {
	frame := 0
	if v, ok := m.views[m.mode]; ok && v.Animating() {
		frame = m.frame
	}
	k := mainKey{
		th: m.th, w: m.lay.mainW, h: m.lay.bodyH, focused: m.focus == regionMain, mode: int(m.mode),
		ver: m.mainVer, loaded: m.loaded, welcome: m.welcome(), frame: frame, meta: m.mainMeta(),
		loadErr: m.loadErr, hasViews: len(m.views),
	}
	if out, ok := m.mainCache.get(k, m.noCache); ok {
		return out
	}
	return m.mainCache.put(k, m.renderMain())
}

// agentKey is everything the agent panel's rendering depends on.
type agentKey struct {
	th       *theme.Theme
	w, h     int
	rail     bool
	focused  bool
	frame    int
	pane     *pane.Pane
	paneVer  uint64
	session  core.Session
	prefix   string
	hasAgent bool
}

func (m *Model) cachedAgent(w int, rail bool, c *regionCache) string {
	k := agentKey{th: m.th, w: w, h: m.lay.bodyH, rail: rail, focused: m.focus == regionAgent, frame: m.frame, prefix: m.prefix}
	if s := m.session(); s != nil {
		k.session = *s
		k.session.UpdatedAt = k.session.UpdatedAt.Truncate(0)
		if p := m.agentPaneOf(s); p != nil {
			k.pane, k.paneVer = p, p.Version()
		}
	}
	if out, ok := c.get(k, m.noCache); ok {
		return out
	}
	return c.put(k, m.renderAgent(w, rail))
}
