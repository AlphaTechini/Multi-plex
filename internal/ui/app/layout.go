package app

import "github.com/alphatechini/multi-plex/internal/ui/view"

// Layout constants (docs/16, "Responsive rules").
const (
	minWidth      = 80 // smallest supported terminal
	minHeight     = 24
	sidebarFull   = 28 // full sidebar
	sidebarRail   = 6  // icon rail
	defaultAgentW = 60 // docs/06: the docked agent panel is never narrower
	agentRailW    = 3  // collapsed agent panel (opens as a drawer)
	mainMinFull   = 88 // main width needed to keep the full sidebar
	mainMinDocked = 72 // main width needed to keep the agent panel docked
)

// region is one of the three framed regions.
type region int

// Regions, in their default left-to-right order.
const (
	regionSidebar region = iota
	regionMain
	regionAgent
)

// layoutInput is everything the layout depends on.
type layoutInput struct {
	w, h         int
	mode         view.Mode
	zoom         bool
	zoomRegion   region
	agentHidden  bool // the user collapsed the docked panel
	agentDrawer  bool // the user opened the drawer over the main view
	agentLeft    bool // the agent panel sits left of the main view
	panelShare   float64
	panelMinCols int
}

// layout is the computed geometry: widths of the three regions (0 when
// hidden), their order and whether the agent panel is a rail or a drawer.
type layout struct {
	w, h, bodyH             int
	tooSmall                bool
	sidebarW, mainW, agentW int
	sidebarRail, agentRail  bool
	drawer                  bool // agent panel drawn as a 60-column overlay
	drawerW                 int
	order                   []region // visible regions, left to right
}

// computeLayout applies the responsive rules: the agent panel takes 30% (at
// least 60 columns); the sidebar collapses to an icon rail first (always in
// Diff mode, where reading width wins); the agent panel becomes a rail, with
// a drawer on demand, when keeping it would leave the main view under 72
// columns.
func computeLayout(in layoutInput) layout {
	l := layout{w: in.w, h: in.h, bodyH: in.h - 1}
	if in.w < minWidth || in.h < minHeight {
		l.tooSmall = true
		return l
	}
	if in.zoom {
		switch in.zoomRegion {
		case regionSidebar:
			l.sidebarW, l.order = in.w, []region{regionSidebar}
		case regionAgent:
			l.agentW, l.order = in.w, []region{regionAgent}
		default:
			l.mainW, l.order = in.w, []region{regionMain}
		}
		return l
	}
	share, minCols := in.panelShare, in.panelMinCols
	if share <= 0 || share >= 1 {
		share = 0.30
	}
	if minCols <= 0 {
		minCols = defaultAgentW
	}
	l.agentW = max(minCols, int(float64(in.w)*share+0.5))
	l.sidebarW = sidebarFull
	if in.mode == view.ModeDiff || in.w-sidebarFull-l.agentW < mainMinFull {
		l.sidebarW, l.sidebarRail = sidebarRail, true
	}
	if in.w-l.sidebarW-l.agentW < mainMinDocked {
		l.agentW, l.agentRail = agentRailW, true
		if in.agentDrawer {
			l.drawer, l.drawerW = true, min(minCols, in.w-l.sidebarW-agentRailW)
		}
	} else if in.agentHidden {
		l.agentW, l.agentRail = agentRailW, true
	}
	l.mainW = in.w - l.sidebarW - l.agentW
	switch {
	case in.agentLeft && !l.agentRail:
		l.order = []region{regionSidebar, regionAgent, regionMain}
	default:
		l.order = []region{regionSidebar, regionMain, regionAgent}
	}
	return l
}

// x returns the left column of region r, or -1 when it is not shown.
func (l layout) x(r region) int {
	x := 0
	for _, o := range l.order {
		if o == r {
			return x
		}
		x += l.width(o)
	}
	return -1
}

// width returns the outer width of region r.
func (l layout) width(r region) int {
	switch r {
	case regionSidebar:
		return l.sidebarW
	case regionAgent:
		return l.agentW
	}
	return l.mainW
}

// regionAt returns the region under column x, and whether any is.
func (l layout) regionAt(x int) (region, bool) {
	if l.drawer && x >= l.w-l.drawerW {
		return regionAgent, true
	}
	c := 0
	for _, r := range l.order {
		c += l.width(r)
		if x < c {
			return r, true
		}
	}
	return regionMain, false
}

// shown reports whether region r takes keyboard focus in this layout: the
// agent rail cannot, the drawer can.
func (l layout) shown(r region) bool {
	if r == regionAgent {
		return !l.agentRail || l.drawer
	}
	return l.width(r) > 0
}

// agentInner returns the agent terminal's size (inside the frame).
func (l layout) agentInner() (cols, rows int) {
	w := l.agentW
	if l.drawer {
		w = l.drawerW
	}
	return max(w-2, 0), max(l.bodyH-2, 0)
}

// mainInner returns the main view's size (inside the frame).
func (l layout) mainInner() (cols, rows int) { return max(l.mainW-2, 0), max(l.bodyH-2, 0) }
