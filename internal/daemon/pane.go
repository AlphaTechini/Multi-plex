package daemon

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/pty"
	"github.com/alphatechini/multi-plex/internal/term"
)

// inputQueue is how many input chunks may wait for one pane's writer.
const inputQueue = 512

// pane is a terminal the daemon hosts for a session.
type pane struct {
	id   core.PaneID
	kind core.PaneKind
	sess *session

	host   atomic.Pointer[term.Host]
	frames atomic.Int64

	mu   sync.Mutex
	info core.Pane

	// Input goes through a queue and one writer goroutine, so a program that
	// has stopped reading its terminal cannot block a client's connection,
	// and keystrokes still arrive in order.
	in   chan []byte
	quit chan struct{}
	once sync.Once
}

// startPane launches a process in a new pane. The pane lives as long as the
// daemon, not the request that started it. ready, if set, runs once the pane
// exists and before its exit can be reported.
func (d *Daemon) startPane(s *session, kind core.PaneKind, l agent.Launch, cols, rows int, ready func(*pane)) (*pane, error) {
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 32
	}
	if len(l.Argv) == 0 {
		return nil, fmt.Errorf("daemon: empty command line for %s pane", kind)
	}
	p := &pane{
		id:   core.PaneID(core.NewID("pane")),
		kind: kind,
		sess: s,
		in:   make(chan []byte, inputQueue),
		quit: make(chan struct{}),
	}
	p.info = core.Pane{ID: p.id, SessionID: s.id, Kind: kind, Cols: cols, Rows: rows}

	d.mu.Lock()
	d.panes[p.id] = p
	d.mu.Unlock()

	spec := pty.Spec{Argv: l.Argv, Dir: l.Dir, Env: l.Env, Cols: cols, Rows: rows}
	host, err := term.Start(d.ctx, d.spawner, d.emulator, spec, p.id, func(f protocol.PaneFrame) { d.onFrame(p, f) })
	if err != nil {
		d.mu.Lock()
		delete(d.panes, p.id)
		d.mu.Unlock()
		return nil, err
	}
	p.host.Store(host)
	go p.writeLoop()
	if ready != nil {
		ready(p) // before OnExit, which runs at once if the process is already gone
	}
	host.OnExit(func(code int) { d.onPaneExit(p, code) })
	return p, nil
}

// writeLoop feeds queued input to the pane's terminal.
func (p *pane) writeLoop() {
	for {
		select {
		case b := <-p.in:
			if h := p.host.Load(); h != nil {
				_ = h.Input(b)
			}
		case <-p.quit:
			return
		}
	}
}

// write queues bytes for the pane's terminal, in order.
func (p *pane) write(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if h := p.host.Load(); h == nil || h.Exited() {
		return fmt.Errorf("pane %s has exited", p.id)
	}
	select {
	case p.in <- b:
		return nil
	case <-p.quit:
		return fmt.Errorf("pane %s is closed", p.id)
	}
}

// shutdown stops the writer and kills the process.
func (p *pane) shutdown() {
	p.once.Do(func() { close(p.quit) })
	if h := p.host.Load(); h != nil {
		_ = h.Close()
	}
}

// snapshotInfo returns the pane's record.
func (p *pane) snapshotInfo() core.Pane {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.info
}

// onFrame publishes a screen update, and treats the first output of an agent
// without hooks as the sign that it is working.
func (d *Daemon) onFrame(p *pane, f protocol.PaneFrame) {
	d.bus.PublishFrame(f)
	if p.kind != core.PaneAgent || p.frames.Add(1) != 2 {
		return
	}
	a, err := adapterFor(p.sess.snap())
	if err != nil || a.Capabilities().Hooks {
		return
	}
	d.update(p.sess, func(c *core.Session) {
		if c.State == core.StateStarting && c.AgentPane == p.id {
			c.State, c.StateDetail = core.StateWorking, ""
		}
	})
}

// onPaneExit records that a pane's process ended.
func (d *Daemon) onPaneExit(p *pane, code int) {
	p.mu.Lock()
	p.info.Exited, p.info.ExitCode = true, code
	info := p.info
	p.mu.Unlock()
	d.publish(protocol.TopicPaneExited, info)
	if p.kind == core.PaneAgent && d.ctx.Err() == nil {
		d.agentExited(p, code)
	}
}

// agentExited moves the session to idle or error after its agent ended on
// its own. A stop requested by the user already set the state.
func (d *Daemon) agentExited(p *pane, code int) {
	s := p.sess
	s.mu.Lock()
	current := s.info.AgentPane == p.id
	stopping := s.stopping
	s.mu.Unlock()
	if !current || stopping {
		return
	}
	d.ports.Release(s.id)
	d.update(s, func(c *core.Session) {
		if code == 0 {
			c.State, c.StateDetail = core.StateIdle, "agent exited"
		} else {
			c.State, c.StateDetail = core.StateError, fmt.Sprintf("agent exited with code %d", code)
		}
	})
}

// paneByID finds a pane.
func (d *Daemon) paneByID(id core.PaneID) (*pane, error) {
	d.mu.RLock()
	p := d.panes[id]
	d.mu.RUnlock()
	if p == nil {
		return nil, notFound("no such pane %q", id)
	}
	return p, nil
}

// paneSnapshot is the bus's source of full frames for lagging subscribers.
func (d *Daemon) paneSnapshot(id core.PaneID) (protocol.PaneFrame, bool) {
	d.mu.RLock()
	p := d.panes[id]
	d.mu.RUnlock()
	if p == nil {
		return protocol.PaneFrame{}, false
	}
	h := p.host.Load()
	if h == nil {
		return protocol.PaneFrame{}, false
	}
	return h.Snapshot(), true
}

// removePane forgets a pane and kills its process.
func (d *Daemon) removePane(p *pane) {
	d.mu.Lock()
	delete(d.panes, p.id)
	d.mu.Unlock()
	p.shutdown()
}

// panesOf lists a session's panes.
func (d *Daemon) panesOf(s *session) []*pane {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []*pane
	for _, p := range d.panes {
		if p.sess == s {
			out = append(out, p)
		}
	}
	return out
}
