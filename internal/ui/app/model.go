package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/pane"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
	"github.com/alphatechini/multi-plex/internal/ui/view"
)

// Model is the shell's Bubble Tea model. It is used from the Bubble Tea loop
// only.
type Model struct {
	o      Options
	th     *theme.Theme
	be     backend.Backend
	env    view.Env
	prefix string
	probe  func(core.Workspace) workspaceInfo
	now    func() time.Time

	w, h int
	lay  layout
	// lastMain and lastAgent are the sizes last sent to the main view and the
	// agent pane, so a resize is announced once.
	lastMain, lastAgent [2]int

	ws       core.Workspace
	loaded   bool
	sessions []core.Session
	cur      core.SessionID // the session the main view belongs to
	notes    map[core.SessionID]int
	specs    []protocol.SpecInfo
	info     workspaceInfo

	mode       view.Mode
	views      map[view.Mode]view.View
	told       map[view.Mode]core.SessionID // session each view was last told
	viewFocus  map[view.Mode]bool
	focus      region
	zoom       bool
	agentHides bool
	agentDraw  bool
	agentLeft  bool

	sb sidebarState

	agentPanes map[core.PaneID]*pane.Pane

	approvals []pendingApproval
	overlays  []overlay
	toasts    []toast
	toastSeq  int

	prefixOn bool

	frame     int
	tickArmed bool
	blurred   bool
	// pendingSelect is a slug to select once its session appears (after the
	// user created it).
	pendingSelect string

	loadErr string
	quit    bool

	bench benchState

	// Region caches: each region's rendered string is reused while its key
	// is unchanged, so streaming agent output redraws only the agent panel.
	// ver counts changes to shell state (anything but ticks and pane
	// frames); mainVer counts messages delivered to the active view.
	ver, mainVer uint64
	noCache      bool
	sbCache      regionCache
	mainCache    regionCache
	agentCache   regionCache
	drawerCache  regionCache

	// noPump disables the event pump command (tests feed events by hand).
	noPump bool
}

// benchState implements Options.BenchFirstFrame.
type benchState struct {
	on    bool
	drawn chan struct{}
	once  bool
}

// newModel builds the shell around opts and theme th.
func newModel(o Options, th *theme.Theme, probe func(core.Workspace) workspaceInfo) *Model {
	if probe == nil {
		probe = probeWorkspace
	}
	m := &Model{
		o: o, th: th, be: o.Backend, prefix: o.prefix(), probe: probe, now: time.Now,
		notes: map[core.SessionID]int{}, views: map[view.Mode]view.View{},
		told: map[view.Mode]core.SessionID{}, viewFocus: map[view.Mode]bool{},
		focus: regionMain, agentPanes: map[core.PaneID]*pane.Pane{},
		bench: benchState{on: o.BenchFirstFrame, drawn: make(chan struct{})},
	}
	m.env = view.Env{Theme: th, Backend: o.Backend, Workspace: core.Workspace{Root: o.WorkspaceDir}}
	return m
}

// Init starts the data load and the event stream.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.loadCmd(), m.waitEvents(), clockCmd())
}

// view returns (building on first use) the view of a mode.
func (m *Model) viewOf(mode view.Mode) view.View {
	v, ok := m.views[mode]
	if !ok {
		v = modeConstructors[mode](m.env)
		m.views[mode] = v
	}
	return v
}

// active is the current mode's view.
func (m *Model) active() view.View { return m.viewOf(m.mode) }

// session returns the current session, or nil.
func (m *Model) session() *core.Session { return m.sessionByID(m.cur) }

func (m *Model) sessionByID(id core.SessionID) *core.Session {
	if id == "" {
		return nil
	}
	for i := range m.sessions {
		if m.sessions[i].ID == id {
			return &m.sessions[i]
		}
	}
	return nil
}

// welcome reports whether the main view shows the first-run welcome screen.
func (m *Model) welcome() bool { return len(m.sessions) == 0 && m.mode == view.ModeOverview }

// agentPane returns the pane component of a session's agent, if attached.
func (m *Model) agentPaneOf(s *core.Session) *pane.Pane {
	if s == nil || s.AgentPane == "" {
		return nil
	}
	return m.agentPanes[s.AgentPane]
}
