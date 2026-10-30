package daemon

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/alphatechini/multi-plex/internal/approval/broker"
	"github.com/alphatechini/multi-plex/internal/config"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/pty"
	"github.com/alphatechini/multi-plex/internal/safety/gate"
	"github.com/alphatechini/multi-plex/internal/store"
	"github.com/alphatechini/multi-plex/internal/vcs"
	"github.com/alphatechini/multi-plex/internal/vt"
)

// Daemon owns the sessions, panes and connections of one Multiplex daemon.
type Daemon struct {
	store    store.Store
	vcs      vcs.VCS
	spawner  pty.Spawner
	emulator vt.Factory
	broker   *broker.Broker
	remotes  Remotes
	paths    config.Paths
	settings config.Settings
	mpxPath  string
	hookCmd  []string // argv prefix agents run for hooks; see HookCommand
	version  string
	approver string
	nowFn    func() time.Time

	log      *slog.Logger
	logClose io.Closer
	bus      *bus
	routes   map[string]route

	// ctx lives as long as the daemon; panes and background work use it, not
	// the context of the request that started them.
	ctx    context.Context
	cancel context.CancelFunc

	mu         sync.RWMutex // guards the maps below
	sessions   map[core.SessionID]*session
	workspaces map[core.WorkspaceID]core.Workspace
	panes      map[core.PaneID]*pane
	conns      map[*conn]struct{}
	addr       string

	createMu sync.Mutex // serialises session creation (slug and branch choice)
	ports    portAlloc
	rate     gate.RateCounter

	stateMu sync.Mutex    // guards stateCh
	stateCh chan struct{} // closed and replaced on every session change

	bg       sync.WaitGroup
	bgOnce   sync.Once
	stopOnce sync.Once
	stopCh   chan struct{} // closed by Shutdown
	doneCh   chan struct{} // closed when Serve has finished shutting down
}

// New builds a daemon from its dependencies, loads the sessions saved in the
// store (sessions that were active become idle: their agents died with the
// previous daemon) and wires the approval broker to the event bus and the
// audit log. Call Serve to accept connections.
func New(dp Deps) *Daemon {
	d := &Daemon{
		store:    dp.Store,
		vcs:      dp.VCS,
		spawner:  dp.Spawner,
		emulator: dp.Emulator,
		broker:   dp.Broker,
		remotes:  dp.Remotes,
		paths:    dp.Paths,
		settings: dp.Settings,
		mpxPath:  dp.MpxPath,
		hookCmd:  HookCommand(dp.MpxPath),
		version:  dp.Version,
		approver: dp.ApproverName,
		nowFn:    dp.Now,

		sessions:   map[core.SessionID]*session{},
		workspaces: map[core.WorkspaceID]core.Workspace{},
		panes:      map[core.PaneID]*pane{},
		conns:      map[*conn]struct{}{},
		stateCh:    make(chan struct{}),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
	if d.nowFn == nil {
		d.nowFn = time.Now
	}
	d.log = dp.Logger
	d.logClose = nopCloser{}
	if d.log == nil {
		d.log, d.logClose = NewFileLogger(dp.Paths.LogDir)
	}
	if d.approver == "" {
		d.approver = "typed-code"
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.bus = newBus(d.log)
	d.bus.snapshot = d.paneSnapshot
	d.routes = d.buildRoutes()
	d.wireBroker()
	d.loadSessions()
	return d
}

func (d *Daemon) now() time.Time { return d.nowFn() }

// Addr returns the address the daemon is serving on, or "" before Serve.
func (d *Daemon) Addr() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.addr
}

// loadSessions restores sessions from the store after a restart.
func (d *Daemon) loadSessions() {
	ctx := d.ctx
	list, err := d.store.ListSessions(ctx, "")
	if err != nil {
		d.log.Error("load sessions", "err", err)
		return
	}
	for _, info := range list {
		if info.State.Active() {
			info.State = core.StateIdle
			info.StateDetail = "agent stopped when the daemon restarted"
			info.UpdatedAt = d.now()
		}
		info.AgentPane = ""
		if err := d.store.PutSession(ctx, info); err != nil {
			d.log.Error("save restored session", "session", info.ID, "err", err)
		}
		d.sessions[info.ID] = newSession(info)
	}
}

// publish sends a non-frame event to subscribers.
func (d *Daemon) publish(topic string, payload any) { d.bus.Publish(topic, payload) }

// toast tells subscribed UIs something short.
func (d *Daemon) toast(level, text string) {
	d.publish(protocol.TopicToast, protocol.Toast{Level: level, Text: text})
}

// background runs fn as a daemon-lifetime goroutine tracked for shutdown.
func (d *Daemon) background(fn func()) {
	d.bg.Add(1)
	go func() {
		defer d.bg.Done()
		defer func() {
			if r := recover(); r != nil {
				d.log.Error("background task panicked", "panic", r)
			}
		}()
		fn()
	}()
}

// serveListener is shared by Serve and tests that hand in a listener.
func (d *Daemon) setAddr(l net.Listener) {
	d.mu.Lock()
	d.addr = l.Addr().String()
	d.mu.Unlock()
}
