package main

// Layout widths (docs/16, "Responsive rules").
const (
	sidebarFull    = 28 // full sidebar
	sidebarRail    = 6  // icon rail
	agentMin       = 60 // docs/06: the agent panel is never narrower
	agentRail      = 3  // collapsed agent panel (opens as a drawer)
	mainMinFull    = 88 // main width needed to keep the full sidebar
	mainMin        = 72 // main width needed to keep the agent panel docked
	fileRailWidth  = 32 // diff file rail
	fileRailMinFor = 112
)

// frame is the computed region layout for one screen size.
type frame struct {
	sidebar, main, agent int
	bodyH                int // height above the status bar
	sidebarRail          bool
	agentRail            bool
}

// computeLayout applies the responsive rules: the agent panel takes 30% (at
// least 60 columns), the sidebar collapses to an icon rail first (always in
// Diff mode, where reading width wins), and the agent panel becomes a drawer
// when the main view would drop below 72 columns.
func computeLayout(w, h int, diffMode bool) frame {
	f := frame{bodyH: h - 1}
	f.agent = max(agentMin, (w*3+5)/10)
	f.sidebar = sidebarFull
	if diffMode || w-sidebarFull-f.agent < mainMinFull {
		f.sidebar, f.sidebarRail = sidebarRail, true
	}
	if w-f.sidebar-f.agent < mainMin {
		f.agent, f.agentRail = agentRail, true
	}
	f.main = w - f.sidebar - f.agent
	return f
}
