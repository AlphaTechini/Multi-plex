// Package pane renders a daemon-hosted terminal pane (the agent's CLI or a
// shell) inside the TUI. It keeps the latest screen rows pushed as
// protocol.PaneFrame events, draws them clipped and padded onto the panel
// surface, forwards keys and paste as bytes, and tells the daemon about size
// changes (debounced). The agent panel and the Term mode both use it.
package pane

import (
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/ui/backend"
	"github.com/alphatechini/multi-plex/internal/ui/theme"
)

// ResizeDebounce is how long a size change waits for further changes before
// it is sent to the daemon.
const ResizeDebounce = 50 * time.Millisecond

// Options configures a Pane.
type Options struct {
	// Theme provides the surface colour and glyphs.
	Theme *theme.Theme
	// Backend carries input, resize and snapshot calls.
	Backend backend.Backend
	// Name is what the pane runs, shown when it exits ("agent", "shell").
	Name string
}

// Pane is the client-side view of one daemon pane. It is used from the
// Bubble Tea loop only (except the input drain goroutine, which touches just
// the pending-input buffer).
type Pane struct {
	th   *theme.Theme
	be   backend.Backend
	name string

	id       core.PaneID
	rows     []string
	cols     int
	nrows    int
	cx, cy   int
	cursorOn bool
	title    string
	exited   bool
	exitCode int

	gotFull  bool // a Full frame was applied since Attach
	sawFrame bool // any frame arrived since the snapshot was requested
	version  uint64

	wantCols, wantRows int
	sentCols, sentRows int
	resizeSeq          int

	inMu     sync.Mutex
	pending  []byte
	draining bool

	cache renderCache
}

// New creates an empty pane. Call Attach to bind it to a daemon pane.
func New(o Options) *Pane {
	name := o.Name
	if name == "" {
		name = "process"
	}
	return &Pane{th: o.Theme, be: o.Backend, name: name}
}

// ID returns the attached pane's ID ("" when none).
func (p *Pane) ID() core.PaneID { return p.id }

// Title returns the title the program in the pane last set.
func (p *Pane) Title() string { return p.title }

// Exited reports whether the pane's process has exited, and its exit code.
func (p *Pane) Exited() (bool, int) { return p.exited, p.exitCode }

// Size returns the daemon-side screen size of the last applied frame.
func (p *Pane) Size() (cols, rows int) { return p.cols, p.nrows }

// Row returns screen row y as last received (ANSI-styled), or "".
func (p *Pane) Row(y int) string {
	if y < 0 || y >= len(p.rows) {
		return ""
	}
	return p.rows[y]
}

// Version changes whenever what Render draws may have changed (frames, exit,
// attach). Callers use it as a cache key for the rendered pane.
func (p *Pane) Version() uint64 { return p.version }

// Attach binds the pane to a daemon pane and fetches its current screen. It
// returns the command that does the fetch; route the resulting SnapshotMsg
// to Update. Attaching the pane it is already bound to is a no-op.
func (p *Pane) Attach(id core.PaneID) tea.Cmd {
	if id == "" || id == p.id {
		return nil
	}
	p.id = id
	p.rows, p.cols, p.nrows = nil, 0, 0
	p.cx, p.cy, p.cursorOn = 0, 0, false
	p.exited, p.exitCode, p.title = false, 0, ""
	p.gotFull, p.sawFrame = false, false
	p.sentCols, p.sentRows = 0, 0
	p.version++
	return p.snapshotCmd()
}

// MarkExited records that the pane's process ended.
func (p *Pane) MarkExited(info core.Pane) {
	if info.ID != p.id {
		return
	}
	p.exited, p.exitCode = info.Exited, info.ExitCode
	p.version++
}

// Apply merges a pane.frame event: a full frame replaces the screen, a
// partial one replaces its changed rows. Frames for other panes are ignored.
func (p *Pane) Apply(f protocol.PaneFrame) {
	if f.PaneID != p.id || p.id == "" {
		return
	}
	p.sawFrame = true
	if f.Full {
		p.gotFull = true
	}
	p.apply(f)
}

func (p *Pane) apply(f protocol.PaneFrame) {
	if f.Full || f.Cols != p.cols || f.Rows != p.nrows || len(p.rows) != f.Rows {
		old := p.rows
		p.rows = make([]string, max(f.Rows, 0))
		if !f.Full {
			copy(p.rows, old)
		}
		p.cols, p.nrows = f.Cols, f.Rows
	}
	for _, r := range f.Changed {
		if r.Y >= 0 && r.Y < len(p.rows) {
			p.rows[r.Y] = r.Text
		}
	}
	p.cx, p.cy, p.cursorOn = f.CursorX, f.CursorY, f.CursorVisible
	if f.Title != "" {
		p.title = f.Title
	}
	p.version++
}
